package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func TestEnsureMemory_SeedsAllCategories(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := EnsureMemory(); err != nil {
		t.Fatalf("EnsureMemory: %v", err)
	}

	memDir := filepath.Join(tmp, ".claude", "memory")
	rulesDir := filepath.Join(tmp, ".claude", "rules")

	if _, err := os.Stat(memDir); err != nil {
		t.Errorf("memory dir not created: %v", err)
	}
	if _, err := os.Stat(rulesDir); err != nil {
		t.Errorf("rules dir not created: %v", err)
	}

	for _, cat := range config.MemoryCategories {
		path := filepath.Join(memDir, cat.Name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("category %s not seeded: %v", cat.Name, err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, "category: "+cat.Name) {
			t.Errorf("category %s missing 'category:' frontmatter field", cat.Name)
		}
		if !strings.Contains(content, "memory_type: "+cat.MemoryType) {
			t.Errorf("category %s missing 'memory_type:' frontmatter field", cat.Name)
		}
		if !strings.Contains(content, "summary: "+cat.Summary) {
			t.Errorf("category %s missing 'summary:' frontmatter field", cat.Name)
		}
		if !strings.Contains(content, "max_lines: 80") {
			t.Errorf("category %s missing 'max_lines:' frontmatter field", cat.Name)
		}
		if !strings.HasPrefix(content, "---\n") {
			t.Errorf("category %s does not start with frontmatter '---'", cat.Name)
		}
	}
}

func TestEnsureMemory_SkipExisting(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	memDir := filepath.Join(tmp, ".claude", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	existing := filepath.Join(memDir, "about-me.md")
	customContent := "my custom content that must not be overwritten"
	if err := os.WriteFile(existing, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureMemory(); err != nil {
		t.Fatalf("EnsureMemory: %v", err)
	}

	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read existing: %v", err)
	}
	if string(data) != customContent {
		t.Errorf("existing file was clobbered; got %q, want %q", string(data), customContent)
	}
}

func TestEnsureMemory_DirsExist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := EnsureMemory(); err != nil {
		t.Fatalf("EnsureMemory: %v", err)
	}

	dirs := []string{
		filepath.Join(tmp, ".claude", "memory"),
		filepath.Join(tmp, ".claude", "rules"),
	}
	for _, d := range dirs {
		fi, err := os.Stat(d)
		if err != nil {
			t.Errorf("dir %s not created: %v", d, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}
}
