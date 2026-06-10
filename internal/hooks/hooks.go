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

func SessionStart() error {
	_, _ = io.ReadAll(os.Stdin)

	memDir := filepath.Join(os.Getenv("HOME"), ".claude", "memory")
	if h, _ := os.UserHomeDir(); os.Getenv("HOME") == "" {
		memDir = filepath.Join(h, ".claude", "memory")
	}

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
