package provision

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func EnsureMemoryRules(cfg config.Config) error {
	src := cfg.MemoryRulesDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil
	}

	dst := filepath.Join(claudeDir(), "rules")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: mkdir rules: %v\n", err)
		return nil
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		dstPath := filepath.Join(dst, e.Name())
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), dstPath); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: copy rule %s: %v\n", e.Name(), err)
		}
	}
	return nil
}
