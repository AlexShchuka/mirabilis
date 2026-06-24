package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const (
	headroomVenvRel   = ".headroom-venv/bin/headroom"
	headroomPollLimit = 60

	postToolUseFailureBulletCap = 10
	postToolUseFailureByteCap   = 2048
)

var (
	runner           exec.Runner = exec.NewHost()
	headroomStatsURL             = config.HeadroomStatsURL()
)

var eventNameRe = regexp.MustCompile(`"hook_event_name"\s*:\s*"([^"]*)"`)

func Dispatch(name string) error {
	switch name {
	case "telegram":
		return Telegram()
	case "session-start":
		return SessionStart()
	case "session-end":
		return SessionEnd()
	case "post-tool-use-failure":
		return PostToolUseFailure()
	default:
		return fmt.Errorf("unknown hook: %s", name)
	}
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
