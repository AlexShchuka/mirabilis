package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func titleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func EnsureMemory() error {
	memDir := filepath.Join(claudeDir(), "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: mkdir memory: %v\n", err)
	}
	rulesDir := filepath.Join(claudeDir(), "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: mkdir rules: %v\n", err)
	}
	for _, cat := range config.MemoryCategories {
		path := filepath.Join(memDir, cat.Name+".md")
		if _, err := os.Stat(path); err == nil {
			continue
		}
		content := "---\ncategory: " + cat.Name + "\nmemory_type: " + cat.MemoryType + "\nsummary: " + cat.Summary + "\nmax_lines: 80\n---\n\n# " + titleCase(cat.Name) + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: write memory %s: %v\n", cat.Name, err)
		}
	}
	return nil
}
