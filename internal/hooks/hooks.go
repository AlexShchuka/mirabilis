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

func Telegram() error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chat := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chat == "" {
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

	client := &http.Client{Timeout: 10 * time.Second}
	endpoint := telegramAPI + "/bot" + token + "/sendMessage"
	form := url.Values{}
	form.Set("chat_id", chat)
	form.Set("text", text)
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: telegram sendMessage: %v\n", err)
		return nil
	}
	resp.Body.Close()
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

func memoryIndex(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}

	type fileInfo struct {
		meta     memoryMeta
		fileName string
		count    int
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
		fi := fileInfo{meta: meta, count: count, fileName: e.Name()}
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
		fmt.Fprintf(&sb, "- **%s** (%s, %d) — %s  · memory/%s\n",
			fi.meta.category, fi.meta.memoryType, fi.count, fi.meta.summary, fi.fileName)
	}
	for _, fi := range extras {
		fmt.Fprintf(&sb, "- **%s** (%s, %d) — %s  · memory/%s\n",
			fi.meta.category, fi.meta.memoryType, fi.count, fi.meta.summary, fi.fileName)
	}
	return sb.String(), nil
}

func sessionHome() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
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
	writeSettingsJSON(path, m)
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
	writeSettingsJSON(path, m)
}

func writeSettingsJSON(path string, m map[string]any) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: marshal settings: %v\n", err)
		return
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: create temp settings: %v\n", err)
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "[hook] WARN: write temp settings: %v\n", err)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "[hook] WARN: close temp settings: %v\n", err)
		return
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "[hook] WARN: chmod temp settings: %v\n", err)
		return
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "[hook] WARN: rename settings: %v\n", err)
	}
}

func headroomBin() string {
	return filepath.Join(sessionHome(), ".headroom-venv", "bin", "headroom")
}

func ensureProxyForSession() {
	h := sessionHome()
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

func SessionStart() error {
	_, _ = io.ReadAll(os.Stdin)

	ensureProxyForSession()

	memDir := filepath.Join(sessionHome(), ".claude", "memory")

	idx, _ := memoryIndex(memDir)
	if idx == "" {
		payload, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "SessionStart",
				"additionalContext": "",
			},
		})
		_, _ = os.Stdout.Write(payload)
		return nil
	}

	if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(idx), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: write MEMORY.md: %v\n", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": idx,
		},
	})
	_, _ = os.Stdout.Write(payload)
	return nil
}

func Dispatch(name string) error {
	switch name {
	case "telegram":
		return Telegram()
	case "session-start":
		return SessionStart()
	default:
		return fmt.Errorf("unknown hook: %s", name)
	}
}
