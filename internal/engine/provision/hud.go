package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type hudStep struct {
	d Deps
}

func (s *hudStep) Meta() pipeline.Meta { return carryMeta("claude-hud", "claude-hud config") }

func (s *hudStep) dest() string {
	return filepath.Join(s.d.claudeDir(), "plugins", "claude-hud", "config.json")
}

func (s *hudStep) Check(_ context.Context) (bool, error) {
	if !exists(s.d.Cfg.HudConfigSeed()) {
		return true, nil
	}
	return exists(s.dest()), nil
}

func (s *hudStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	seed := s.d.Cfg.HudConfigSeed()
	if !exists(seed) {
		return nil
	}
	dest := s.dest()
	if exists(dest) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir claude-hud config dir: %w", err)
	}
	return copyFile(seed, dest)
}
