package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type rtkStep struct {
	d Deps
}

func (s *rtkStep) Meta() pipeline.Meta { return installMeta("rtk", "rtk gateway") }

func (s *rtkStep) Check(ctx context.Context) (bool, error) {
	if !s.d.argvOK(ctx, "rtk", "--version") {
		return true, nil
	}
	return rtkHookPresent(s.d), nil
}

func (s *rtkStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.argvOK(ctx, "rtk", "--version") {
		return nil
	}
	if rtkHookPresent(s.d) {
		return nil
	}
	if err := s.d.stream(ctx, "rtk", out, "timeout", "60", "rtk", "init", "-g", "--auto-patch"); err != nil {
		return fmt.Errorf("rtk init: %w", err)
	}
	return nil
}

func rtkHookPresent(d Deps) bool {
	m, err := readJSON(d.settingsPath())
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
	for _, entry := range toSlice(preToolUse) {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range toSlice(em["hooks"]) {
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

type rtkConfigStep struct {
	d Deps
}

func (s *rtkConfigStep) Meta() pipeline.Meta { return carryMeta("rtk-config", "rtk config") }

func (s *rtkConfigStep) dest() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "rtk", "config.toml")
	}
	return filepath.Join(s.d.Home, ".config", "rtk", "config.toml")
}

func (s *rtkConfigStep) Check(_ context.Context) (bool, error) {
	if !exists(s.d.Cfg.RTKConfigSeed()) {
		return true, nil
	}
	return exists(s.dest()), nil
}

func (s *rtkConfigStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	seed := s.d.Cfg.RTKConfigSeed()
	if !exists(seed) {
		return nil
	}
	dest := s.dest()
	if exists(dest) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir rtk config dir: %w", err)
	}
	return copyFile(seed, dest)
}
