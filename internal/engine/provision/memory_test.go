package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
)

func TestMemoryStepRestoreMovesFilesAndRemovesSnapshot(t *testing.T) {
	d, _ := testDeps(t)
	snap := filepath.Join(d.Repo, ".mirabilis", "saved-memory")
	mustWrite(t, filepath.Join(snap, "about-me.md"), "saved facts\n")
	mustWrite(t, filepath.Join(snap, "research-log.md"), "saved log\n")
	step := &memoryStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false while a snapshot is pending")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(step.memoryDir(), "about-me.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "saved facts\n" {
		t.Errorf("restored content = %q, want saved facts", got)
	}
	if exists(snap) {
		t.Error("snapshot dir should be removed after restore")
	}
	if !checkStep(t, step) {
		t.Error("check should be true after restore+ensure")
	}
}

func TestMemoryStepEnsureCreatesCategoriesIdempotently(t *testing.T) {
	d, _ := testDeps(t)
	step := &memoryStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, cat := range config.MemoryCategories {
		path := filepath.Join(step.memoryDir(), cat.Name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("category %s: %v", cat.Name, err)
		}
		if !strings.Contains(string(data), "category: "+cat.Name) {
			t.Errorf("category %s missing frontmatter", cat.Name)
		}
	}
	if !exists(filepath.Join(d.claudeDir(), "rules")) {
		t.Error("rules dir should be created")
	}
	custom := filepath.Join(step.memoryDir(), "about-me.md")
	mustWrite(t, custom, "user content\n")
	if err := runStep(t, step); err != nil {
		t.Fatalf("second run: %v", err)
	}
	data, err := os.ReadFile(custom)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "user content\n" {
		t.Errorf("existing category overwritten: %q", data)
	}
}

func TestMemoryStepCheckFalseWhenCategoryMissing(t *testing.T) {
	d, _ := testDeps(t)
	step := &memoryStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(step.memoryDir(), "prep.md")); err != nil {
		t.Fatal(err)
	}
	if checkStep(t, step) {
		t.Error("check should be false when a category file is missing")
	}
}
