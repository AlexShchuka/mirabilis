package provision

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func rtkConfigPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "rtk", "config.toml")
	}
	return filepath.Join(Home(), ".config", "rtk", "config.toml")
}

func EnsureRTKConfig(cfg config.Config) error {
	seed := cfg.RTKConfigSeed()
	if _, err := os.Stat(seed); err != nil {
		return nil
	}
	dest := rtkConfigPath()
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("provision: mkdir rtk config dir: %w", err)
	}
	return copyFile(seed, dest)
}
