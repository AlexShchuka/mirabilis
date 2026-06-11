package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
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

func TestRestoreMemory_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repoRoot := t.TempDir()
	savePath := runtime.MemorySavePath(repoRoot)
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	const content = "---\ncategory: sandbox-ops\nmemory_type: procedural\nsummary: s\n---\n\n- bullet one\n"
	if err := os.WriteFile(filepath.Join(savePath, "sandbox-ops.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	RestoreMemory(repoRoot)

	dst := filepath.Join(tmp, ".claude", "memory", "sandbox-ops.md")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("memory file not restored: %v", err)
	}
	if string(data) != content {
		t.Errorf("restored content = %q, want %q", string(data), content)
	}
	if _, err := os.Stat(savePath); err == nil {
		t.Error("saved-memory staging dir should be removed after restore")
	}
}

func TestRestoreMemory_NoSnapshot_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repoRoot := t.TempDir()

	RestoreMemory(repoRoot)

	memDir := filepath.Join(tmp, ".claude", "memory")
	entries, _ := os.ReadDir(memDir)
	if len(entries) != 0 {
		t.Errorf("RestoreMemory with no snapshot should not create any files, got %v", entries)
	}
}

func TestRestoreMemory_DestroysSnapshotAfterRestore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repoRoot := t.TempDir()
	savePath := runtime.MemorySavePath(repoRoot)
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savePath, "about-me.md"), []byte("---\ncategory: about-me\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	RestoreMemory(repoRoot)

	if _, err := os.Stat(savePath); err == nil {
		t.Error("staging dir must be removed after successful restore")
	}
}
