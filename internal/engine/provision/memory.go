package provision

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type memoryStep struct {
	d Deps
}

func (s *memoryStep) Meta() pipeline.Meta { return carryMeta("memory", "Persistent memory") }

func (s *memoryStep) snapshotDir() string {
	return filepath.Join(s.d.Repo, ".mirabilis", "saved-memory")
}

func (s *memoryStep) memoryDir() string { return filepath.Join(s.d.claudeDir(), "memory") }

func (s *memoryStep) Check(_ context.Context) (bool, error) {
	if exists(s.snapshotDir()) {
		return false, nil
	}
	for _, cat := range config.MemoryCategories {
		if !exists(filepath.Join(s.memoryDir(), cat.Name+".md")) {
			return false, nil
		}
	}
	return true, nil
}

func (s *memoryStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	return errors.Join(s.restore(), s.ensure())
}

func (s *memoryStep) restore() error {
	src := s.snapshotDir()
	if !exists(src) {
		return nil
	}
	dst := s.memoryDir()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("restore memory: mkdir: %w", err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("restore memory: readdir: %w", err)
	}
	var errs []error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			errs = append(errs, fmt.Errorf("restore memory: copy %s: %w", e.Name(), err))
		}
	}
	if len(errs) == 0 {
		return os.RemoveAll(src)
	}
	return errors.Join(errs...)
}

func (s *memoryStep) ensure() error {
	memDir := s.memoryDir()
	var errs []error
	errs = append(errs, os.MkdirAll(memDir, 0o755))
	errs = append(errs, os.MkdirAll(filepath.Join(s.d.claudeDir(), "rules"), 0o755))
	for _, cat := range config.MemoryCategories {
		path := filepath.Join(memDir, cat.Name+".md")
		if exists(path) {
			continue
		}
		content := "---\ncategory: " + cat.Name + "\nmemory_type: " + cat.MemoryType +
			"\nsummary: " + cat.Summary + "\nmax_lines: 80\n---\n\n# " + titleCase(cat.Name) + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			errs = append(errs, fmt.Errorf("write memory %s: %w", cat.Name, err))
		}
	}
	return errors.Join(errs...)
}

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
