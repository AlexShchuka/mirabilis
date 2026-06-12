package hooks

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision"
	"github.com/google/uuid"
)

const (
	headroomVenvRel   = ".headroom-venv/bin/headroom"
	headroomStatsURL  = "http://127.0.0.1:8787/stats"
	headroomPollLimit = 60

	postToolUseFailureBulletCap = 10
	postToolUseFailureByteCap   = 2048
)

var runner exec.Runner = exec.NewHost()

var eventNameRe = regexp.MustCompile(`"hook_event_name"\s*:\s*"([^"]*)"`)

func Dispatch(name string) error {
	switch name {
	case "telegram":
		return Telegram()
	case "session-start":
		return SessionStart()
	case "post-tool-use-failure":
		return PostToolUseFailure()
	default:
		return fmt.Errorf("unknown hook: %s", name)
	}
}

func Telegram() error {
	repo := repoRoot()
	chat, err := notify.ReadChatID(repo)
	if err != nil || chat == "" {
		return nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: read stdin: %v\n", err)
		return nil
	}

	text, ok := messageFor(eventName(data))
	if !ok {
		return nil
	}
	if proj := cwdBaseName(data); proj != "" {
		text = strings.Replace(text, "mirabilis:", "mirabilis ["+proj+"]:", 1)
	}

	job := notify.Job{
		ID:        uuid.NewString(),
		ChatID:    chat,
		Text:      text,
		CreatedAt: time.Now().UTC(),
	}
	if err := notify.WriteJob(notify.OutboxDir(repo), job); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: telegram queue: %v\n", err)
	}
	return nil
}

func SessionStart() error {
	_, _ = io.ReadAll(os.Stdin)

	ensureProxyForSession(context.Background())

	memDir := filepath.Join(home(), ".claude", "memory")
	idx, _ := memoryIndex(memDir)
	tgCtx := sessionTelegramContext(repoRoot())

	additionalContext := idx
	if tgCtx != "" {
		if additionalContext != "" {
			additionalContext += "\n" + tgCtx
		} else {
			additionalContext = tgCtx
		}
	}

	if idx != "" {
		if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(idx), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "[hook] WARN: write MEMORY.md: %v\n", err)
		}
	}

	payload, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": additionalContext,
		},
	})
	_, _ = os.Stdout.Write(payload)
	return nil
}

func PostToolUseFailure() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: PostToolUseFailure: read stdin: %v\n", err)
		return nil
	}

	var payload struct {
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
		ToolResponse struct {
			Stdout string `json:"stdout"`
		} `json:"tool_response"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}

	combined := payload.ToolInput.Command + " " + payload.ToolResponse.Stdout
	tokens := tokenize(combined)
	if len(tokens) == 0 {
		return nil
	}

	memDir := filepath.Join(home(), ".claude", "memory")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		return nil
	}

	var matched []string
	totalBytes := 0
outer:
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "MEMORY.md" {
			continue
		}
		fileData, err := os.ReadFile(filepath.Join(memDir, e.Name()))
		if err != nil {
			continue
		}
		for _, bullet := range readBullets(fileData) {
			if !matchesBullet(bullet, tokens) {
				continue
			}
			if len(matched) >= postToolUseFailureBulletCap {
				break outer
			}
			if totalBytes+len(bullet) > postToolUseFailureByteCap {
				break outer
			}
			matched = append(matched, bullet)
			totalBytes += len(bullet)
		}
	}

	if len(matched) == 0 {
		return nil
	}

	additionalContext := "Relevant memory:\n" + strings.Join(matched, "\n")
	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUseFailure",
			"additionalContext": additionalContext,
		},
	})
	_, _ = os.Stdout.Write(out)
	return nil
}

func ensureProxyForSession(ctx context.Context) {
	data, err := os.ReadFile(filepath.Join(home(), ".claude", provision.UpstreamFileName))
	if err != nil {
		return
	}
	if proxyAlive(ctx) {
		return
	}
	if !startHeadroom(ctx, strings.TrimSpace(string(data))) {
		fmt.Fprintln(os.Stderr, "[hook] WARN: headroom proxy not ready")
	}
}

func proxyAlive(ctx context.Context) bool {
	return runScript(ctx, `curl -fsS `+headroomStatsURL+` >/dev/null 2>&1`) == nil
}

func startHeadroom(ctx context.Context, upstream string) bool {
	env := ""
	if upstream != "" {
		env = fmt.Sprintf("ANTHROPIC_TARGET_API_URL=%q ", upstream)
	}
	start := fmt.Sprintf(`%ssetsid nohup %q proxy >"$HOME/.headroom-proxy.log" 2>&1 &`,
		env, filepath.Join(home(), headroomVenvRel))
	if err := runScript(ctx, start); err != nil {
		return false
	}
	poll := fmt.Sprintf(`for i in $(seq 1 %d); do curl -fsS %s >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1`,
		headroomPollLimit, headroomStatsURL)
	return runScript(ctx, poll) == nil
}

func runScript(ctx context.Context, script string) error {
	_, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"bash", "-lc", script}})
	return err
}

func sessionTelegramContext(repo string) string {
	id, err := notify.ReadChatID(repo)
	if err != nil || id == "" {
		return ""
	}
	return "telegram channel: " + id + " (cached)"
}

func eventName(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var v struct {
		HookEventName string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(data, &v); err == nil && v.HookEventName != "" {
		return v.HookEventName
	}
	if m := eventNameRe.FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return ""
}

func cwdBaseName(data []byte) string {
	var v struct {
		CWD string `json:"cwd"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	if v.CWD == "" {
		return ""
	}
	return filepath.Base(v.CWD)
}

func messageFor(event string) (string, bool) {
	switch event {
	case "Notification":
		return "❓ mirabilis: needs your input", true
	case "Stop":
		return "✅ mirabilis: task finished", true
	}
	return "", false
}

func repoRoot() string {
	if repo := os.Getenv("MIRABILIS_REPO"); repo != "" {
		return repo
	}
	return "/workspace"
}

func home() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

type memoryMeta struct {
	category   string
	memoryType string
	summary    string
}

func parseFrontmatter(data []byte) memoryMeta {
	var m memoryMeta
	sc := bufio.NewScanner(bytes.NewReader(data))
	inFront := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			if !inFront {
				inFront = true
				continue
			}
			break
		}
		if !inFront {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "category: "); ok {
			m.category = strings.TrimSpace(rest)
		}
		if rest, ok := strings.CutPrefix(line, "memory_type: "); ok {
			m.memoryType = strings.TrimSpace(rest)
		}
		if rest, ok := strings.CutPrefix(line, "summary: "); ok {
			m.summary = strings.TrimSpace(rest)
		}
	}
	return m
}

func countInvariants(data []byte) int {
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	inFront := false
	pastFront := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			if !inFront {
				inFront = true
				continue
			}
			pastFront = true
			inFront = false
			continue
		}
		if pastFront && strings.HasPrefix(line, "- ") {
			n++
		}
	}
	return n
}

func readBullets(data []byte) []string {
	var bullets []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	pastFront := false
	inFront := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			if !inFront {
				inFront = true
				continue
			}
			pastFront = true
			inFront = false
			continue
		}
		if pastFront && strings.HasPrefix(line, "- ") {
			bullets = append(bullets, line)
		}
	}
	return bullets
}

func memoryIndex(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}

	type fileInfo struct {
		meta     memoryMeta
		fileName string
		count    int
		data     []byte
	}

	knownCats := make(map[string]bool, len(config.MemoryCategories))
	for _, cat := range config.MemoryCategories {
		knownCats[cat.Name] = true
	}

	byCategory := make(map[string]fileInfo, len(config.MemoryCategories))
	var extras []fileInfo

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "MEMORY.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		meta := parseFrontmatter(data)
		count := countInvariants(data)
		fi := fileInfo{meta: meta, count: count, fileName: e.Name(), data: data}
		if knownCats[meta.category] {
			if _, exists := byCategory[meta.category]; !exists {
				byCategory[meta.category] = fi
			}
		} else {
			if meta.category == "" {
				stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				fi.meta.category = stem
			}
			extras = append(extras, fi)
		}
	}

	var sb strings.Builder
	sb.WriteString("# Sandbox memory index\n\n")
	for _, cat := range config.MemoryCategories {
		fi, ok := byCategory[cat.Name]
		if !ok {
			continue
		}
		if fi.meta.category == "sandbox-ops" {
			fmt.Fprintf(&sb, "## sandbox-ops\n\n")
			for _, bullet := range readBullets(fi.data) {
				sb.WriteString(bullet + "\n")
			}
			sb.WriteString("\n")
			continue
		}
		fmt.Fprintf(&sb, "- **%s** (%s, %d) — %s  · memory/%s\n",
			fi.meta.category, fi.meta.memoryType, fi.count, fi.meta.summary, fi.fileName)
	}
	for _, fi := range extras {
		fmt.Fprintf(&sb, "- **%s** (%s, %d) — %s  · memory/%s\n",
			fi.meta.category, fi.meta.memoryType, fi.count, fi.meta.summary, fi.fileName)
	}
	return sb.String(), nil
}

func tokenize(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ':', '/', '\\', '.', ',', ';', '!', '?', '"', '\'', '(', ')', '[', ']', '{', '}', '=', '-', '_':
			return true
		}
		return false
	})
	var out []string
	for _, f := range fields {
		if len(f) >= 3 {
			out = append(out, strings.ToLower(f))
		}
	}
	return out
}

func matchesBullet(bullet string, tokens []string) bool {
	low := strings.ToLower(bullet)
	for _, tok := range tokens {
		if strings.Contains(low, tok) {
			return true
		}
	}
	return false
}
