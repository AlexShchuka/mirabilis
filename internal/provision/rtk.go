package provision

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func EnsureRTK(ctx context.Context, r runner.Runner, cfg config.Config) error {
	if _, err := r.Host(ctx, "rtk", "--version"); err != nil {
		return nil
	}

	if rtkHookPresent() {
		return nil
	}

	if _, err := r.Host(ctx, "timeout", "60", "rtk", "init", "-g", "--auto-patch"); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: rtk init: %v\n", err)
	}
	return nil
}

func rtkHookPresent() bool {
	dest := settingsPath()
	m, err := readJSON(dest)
	if err != nil {
		return false
	}

	hooks, ok := m["hooks"].(map[string]any)
	if !ok {
		return false
	}
	preToolUse, ok := hooks["PreToolUse"]
	if !ok {
		return false
	}

	entries := toSlice(preToolUse)
	for _, entry := range entries {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner := toSlice(em["hooks"])
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			if strings.TrimSpace(cmd) == "rtk hook claude" {
				return true
			}
		}
	}
	return false
}

func toSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
