package provision

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
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

func RestoreMemory(repoRoot string) {
	src := runtime.MemorySavePath(repoRoot)
	if _, err := os.Stat(src); err != nil {
		return
	}
	dst := filepath.Join(claudeDir(), "memory")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		warn("restore memory: mkdir", err)
		return
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		warn("restore memory: readdir", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if copyMemFile(srcPath, dstPath) == nil {
			continue
		}
	}
	_ = os.RemoveAll(src)
}

func copyMemFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func EnsureMemory() error {
	memDir := filepath.Join(claudeDir(), "memory")
	warn("mkdir memory", os.MkdirAll(memDir, 0o755))
	rulesDir := filepath.Join(claudeDir(), "rules")
	warn("mkdir rules", os.MkdirAll(rulesDir, 0o755))
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
