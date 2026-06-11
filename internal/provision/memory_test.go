package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func TestRestoreMemory_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := t.TempDir()

	savePath := filepath.Join(root, ".mirabilis", "saved-memory")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	const content1 = "---\ncategory: sandbox-ops\n---\n\n- bullet one\n"
	const content2 = "---\ncategory: about-me\n---\n\n- bullet two\n"
	if err := os.WriteFile(filepath.Join(savePath, "sandbox-ops.md"), []byte(content1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savePath, "about-me.md"), []byte(content2), 0o644); err != nil {
		t.Fatal(err)
	}

	RestoreMemory(root)

	memDir := filepath.Join(tmp, ".claude", "memory")
	for name, want := range map[string]string{"sandbox-ops.md": content1, "about-me.md": content2} {
		data, err := os.ReadFile(filepath.Join(memDir, name))
		if err != nil {
			t.Errorf("memory file %s not restored: %v", name, err)
			continue
		}
		if string(data) != want {
			t.Errorf("restored %s = %q, want %q", name, string(data), want)
		}
	}
	if _, err := os.Stat(savePath); err == nil {
		t.Error("snapshot dir must be removed after restore")
	}
}

func TestRestoreMemory_NoSnapshot_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := t.TempDir()

	RestoreMemory(root)

	memDir := filepath.Join(tmp, ".claude", "memory")
	entries, _ := os.ReadDir(memDir)
	if len(entries) != 0 {
		t.Errorf("RestoreMemory with no snapshot created files: %v", entries)
	}
}

func TestRestoreMemory_OverwritesExistingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	root := t.TempDir()

	memDir := filepath.Join(tmp, ".claude", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(memDir, "sandbox-ops.md")
	if err := os.WriteFile(existingPath, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	savePath := filepath.Join(root, ".mirabilis", "saved-memory")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	const restored = "---\ncategory: sandbox-ops\n---\n\n- restored bullet\n"
	if err := os.WriteFile(filepath.Join(savePath, "sandbox-ops.md"), []byte(restored), 0o644); err != nil {
		t.Fatal(err)
	}

	RestoreMemory(root)

	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("memory file gone after restore: %v", err)
	}
	if string(data) != restored {
		t.Errorf("file not overwritten: got %q, want %q", string(data), restored)
	}
}

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
