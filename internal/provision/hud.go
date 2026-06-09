package provision

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func hudConfigPath() string {
	return filepath.Join(claudeDir(), "plugins", "claude-hud", "config.json")
}

func EnsureHudConfig(cfg config.Config) error {
	seed := cfg.HudConfigSeed()
	if _, err := os.Stat(seed); err != nil {
		return nil
	}
	dest := hudConfigPath()
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("provision: mkdir claude-hud config dir: %w", err)
	}
	return copyFile(seed, dest)
}
