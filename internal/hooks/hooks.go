package hooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/telegram"
	"github.com/google/uuid"
)

var telegramAPI = "https://api.telegram.org"

var eventNameRe = regexp.MustCompile(`"hook_event_name"\s*:\s*"([^"]*)"`)

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

func telegramQueueDir() string {
	repo := os.Getenv("MIRABILIS_REPO")
	if repo == "" {
		repo = "/workspace"
	}
	return telegram.OutboxDir(repo)
}

func Telegram() error {
	chat := os.Getenv("TELEGRAM_CHAT_ID")
	if chat == "" {
		return nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: read stdin: %v\n", err)
		return nil
	}

	name := eventName(data)
	text, ok := messageFor(name)
	if !ok {
		return nil
	}

	if proj := cwdBaseName(data); proj != "" {
		text = strings.Replace(text, "mirabilis:", "mirabilis ["+proj+"]:", 1)
	}

	job := telegram.Job{
		ID:        uuid.NewString(),
		ChatID:    chat,
		Text:      text,
		CreatedAt: time.Now().UTC(),
	}
	if err := telegram.WriteJob(telegramQueueDir(), job); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: telegram queue: %v\n", err)
	}
	return nil
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

func proxyAlive() bool {
	resp, err := http.Get("http://127.0.0.1:8787/stats")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func sessionStartBaseURL(path string) {
	f, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(f)))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return
	}
	env, _ := m["env"].(map[string]any)
	if env == nil {
		env = make(map[string]any)
	}
	env["ANTHROPIC_BASE_URL"] = "http://127.0.0.1:8787"
	m["env"] = env
	if err := provision.WriteJSON(path, m); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: write settings: %v\n", err)
	}
}

func sessionRemoveBaseURL(path string) {
	f, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(f)))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return
	}
	env, _ := m["env"].(map[string]any)
	if env == nil || env["ANTHROPIC_BASE_URL"] == nil {
		return
	}
	delete(env, "ANTHROPIC_BASE_URL")
	m["env"] = env
	if err := provision.WriteJSON(path, m); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: write settings: %v\n", err)
	}
}

func headroomBin() string {
	return filepath.Join(provision.Home(), ".headroom-venv", "bin", "headroom")
}

func ensureProxyForSession() {
	h := provision.Home()
	sp := filepath.Join(h, ".claude", "settings.json")
	if proxyAlive() {
		sessionStartBaseURL(sp)
		return
	}
	if _, err := os.Stat(headroomBin()); err != nil {
		sessionRemoveBaseURL(sp)
		return
	}
	_ = exec.Command("bash", "-lc",
		`setsid nohup "$HOME/.headroom-venv/bin/headroom" proxy >"$HOME/.headroom-proxy.log" 2>&1 &`).Start()
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		if proxyAlive() {
			sessionStartBaseURL(sp)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "[hook] WARN: headroom proxy not ready; removing ANTHROPIC_BASE_URL\n")
	sessionRemoveBaseURL(sp)
}

// telegramChannelCachePath returns the path to the cached Telegram channel-ID
// state file inside ~/.claude. This file is written by detectAndCacheTelegramChannel
// and read by sessionTelegramContext to inject an additionalContext line.
func telegramChannelCachePath() string {
	return filepath.Join(provision.Home(), ".claude", ".mirabilis-telegram-channel")
}

// detectAndCacheTelegramChannel attempts one getUpdates call to discover the
// channel ID from the first channel_post in the bot's update queue. It is
// called only when the bot token secret exists AND no channel is cached yet.
//
// Fail-soft: any error (no token, network, no posts yet) writes nothing and
// emits at most one brief stderr note. The session NEVER fails because of this.
//
// The token is read from the secret file — it never appears in logs, output,
// or error messages.
func detectAndCacheTelegramChannel(tokenPath, cachePath, apiBaseURL string) {
	// TODO: token source: pending isolation design (issue #115) — this read
	// will move to a broker/keychain call once the isolation model is decided.
	raw, err := os.ReadFile(tokenPath)
	if err != nil {
		// No token file — silently skip.
		return
	}
	token := strings.TrimRight(string(raw), "\r\n")
	if token == "" {
		return
	}

	// Build the URL without the token visible in any string we log.
	apiURL := apiBaseURL + "/bot" + token + "/getUpdates"
	params := url.Values{}
	params.Set("allowed_updates", `["channel_post"]`)
	params.Set("limit", "1")
	params.Set("timeout", "0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(apiURL, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] telegram: channel not detected yet — post anything in the channel\n")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			ChannelPost *struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"channel_post"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil || !result.OK {
		fmt.Fprintf(os.Stderr, "[hook] telegram: channel not detected yet — post anything in the channel\n")
		return
	}
	for _, u := range result.Result {
		if u.ChannelPost == nil {
			continue
		}
		id := u.ChannelPost.Chat.ID
		if id == 0 {
			continue
		}
		// Write the chat ID to the cache file.
		data := fmt.Sprintf("%d\n", id)
		if err := os.WriteFile(cachePath, []byte(data), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "[hook] WARN: write telegram channel cache: %v\n", err)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "[hook] telegram: channel not detected yet — post anything in the channel\n")
}

// sessionTelegramContext returns an additionalContext line for the Telegram
// channel if the channel ID is already cached. Returns empty string otherwise.
// The token is NEVER read or included — only the chat ID.
func sessionTelegramContext(cachePath string) string {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return ""
	}
	id := strings.TrimRight(string(data), "\r\n")
	if id == "" {
		return ""
	}
	return "telegram channel: " + id + " (cached)"
}

func SessionStart() error {
	_, _ = io.ReadAll(os.Stdin)

	ensureProxyForSession()

	// Telegram channel auto-detect: if the bot token secret exists and no
	// channel is cached yet, make one getUpdates call to discover the channel.
	// This is strictly fail-soft — session startup never blocks on this.
	tokenPath := telegramTokenSecretPath()
	cachePath := telegramChannelCachePath()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		detectAndCacheTelegramChannel(tokenPath, cachePath, telegramAPI)
	}

	// Collect additionalContext from memory and from the cached Telegram channel.
	memDir := filepath.Join(provision.Home(), ".claude", "memory")

	idx, _ := memoryIndex(memDir)
	tgCtx := sessionTelegramContext(cachePath)

	additionalContext := idx
	if tgCtx != "" {
		if additionalContext != "" {
			additionalContext += "\n" + tgCtx
		} else {
			additionalContext = tgCtx
		}
	}

	// Write MEMORY.md only when the memory index is non-empty.
	// Gate on idx (not additionalContext) so that a Telegram-only context
	// never truncates MEMORY.md to an empty string.
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

// telegramTokenSecretPath is the single token-source seam for hooks.
// TODO: token source: pending isolation design (issue #115) — replace with a
// broker/keychain call once the isolation model is decided.
//
// telegramTokenSecretPath returns the default path for the bot token secret
// file as expected inside the container. This is the path the container mounts
// via the compose secrets mechanism.
func telegramTokenSecretPath() string {
	const defaultPath = "/run/secrets/telegram_bot_token"
	return defaultPath
}

const (
	postToolUseFailureBulletCap = 10
	postToolUseFailureByteCap   = 2048
)

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

	memDir := filepath.Join(provision.Home(), ".claude", "memory")
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
