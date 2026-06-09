package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryIndex_ContainsCategories(t *testing.T) {
	tmp := t.TempDir()

	writeTestFile(t, tmp, "about-me.md", `---
category: about-me
memory_type: semantic
summary: Stable facts about you: identity, role, goals, hard preferences, constraints.
max_lines: 80
---

# About Me

- I am a software engineer
- I prefer Go
`)

	writeTestFile(t, tmp, "research-log.md", `---
category: research-log
memory_type: episodic
summary: Dated findings tied to a specific investigation, paper, or bug. Append-only, compacted periodically.
max_lines: 80
---

# Research Log

- 2026-06-01: studied CoALA paper
- 2026-06-02: reviewed MemGPT architecture
- 2026-06-03: compared memory approaches
`)

	writeTestFile(t, tmp, "dev-principles.md", `---
category: dev-principles
memory_type: procedural
summary: Cross-project engineering invariants you endorse: style, testing bar, anti-slop.
max_lines: 80
---

# Dev Principles

`)

	idx, err := memoryIndex(tmp)
	if err != nil {
		t.Fatalf("memoryIndex: %v", err)
	}

	if !strings.Contains(idx, "about-me") {
		t.Error("index missing 'about-me'")
	}
	if !strings.Contains(idx, "semantic") {
		t.Error("index missing memory_type 'semantic'")
	}
	if !strings.Contains(idx, "2)") {
		t.Error("index missing invariant count 2 for about-me")
	}
	if !strings.Contains(idx, "memory/about-me.md") {
		t.Error("index missing file path for about-me")
	}

	if !strings.Contains(idx, "research-log") {
		t.Error("index missing 'research-log'")
	}
	if !strings.Contains(idx, "episodic") {
		t.Error("index missing memory_type 'episodic'")
	}
	if !strings.Contains(idx, "3)") {
		t.Error("index missing invariant count 3 for research-log")
	}
	if !strings.Contains(idx, "memory/research-log.md") {
		t.Error("index missing file path for research-log")
	}

	if !strings.Contains(idx, "dev-principles") {
		t.Error("index missing 'dev-principles'")
	}
	if !strings.Contains(idx, "procedural") {
		t.Error("index missing memory_type 'procedural'")
	}
	if !strings.Contains(idx, "0)") {
		t.Error("index missing invariant count 0 for dev-principles")
	}
}

func TestMemoryIndex_SkipsMEMORYmd(t *testing.T) {
	tmp := t.TempDir()

	writeTestFile(t, tmp, "MEMORY.md", "# Sandbox memory index\n\nsome content\n")
	writeTestFile(t, tmp, "about-me.md", `---
category: about-me
memory_type: semantic
summary: Stable facts about you.
max_lines: 80
---

# About Me

- fact one
`)

	idx, err := memoryIndex(tmp)
	if err != nil {
		t.Fatalf("memoryIndex: %v", err)
	}

	if strings.Contains(idx, "MEMORY.md") {
		t.Error("index should not reference MEMORY.md as a category")
	}
	if !strings.Contains(idx, "about-me") {
		t.Error("index should still contain about-me")
	}
}

func TestMemoryIndex_MissingDir(t *testing.T) {
	idx, err := memoryIndex("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Errorf("memoryIndex missing dir should return nil error, got %v", err)
	}
	if idx != "" && idx != "# Sandbox memory index\n\n" {
		t.Logf("memoryIndex empty dir returned: %q", idx)
	}
}

func TestDispatchSessionStart_MissingMemoryDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	err := Dispatch("session-start")
	if err != nil {
		t.Errorf("Dispatch(session-start) with missing memory dir returned error: %v", err)
	}
}

func TestMemoryIndex_DuplicateCategory_FirstWins(t *testing.T) {
	tmp := t.TempDir()

	writeTestFile(t, tmp, "a-about-me.md", `---
category: about-me
memory_type: semantic
summary: First file.
max_lines: 80
---

# About Me

- fact one
`)
	writeTestFile(t, tmp, "b-about-me.md", `---
category: about-me
memory_type: semantic
summary: Second file, should be ignored.
max_lines: 80
---

# About Me

- fact two
- fact three
`)

	idx, err := memoryIndex(tmp)
	if err != nil {
		t.Fatalf("memoryIndex: %v", err)
	}

	if !strings.Contains(idx, "First file") {
		t.Error("index should contain summary from first file")
	}
	if strings.Contains(idx, "Second file") {
		t.Error("index should not contain summary from second file (duplicate category)")
	}
}

func TestMemoryIndex_UnknownAndEmptyCategory_AppendedAfter(t *testing.T) {
	tmp := t.TempDir()

	writeTestFile(t, tmp, "about-me.md", `---
category: about-me
memory_type: semantic
summary: Known category.
max_lines: 80
---

# About Me

- fact one
`)
	writeTestFile(t, tmp, "custom-notes.md", `---
category: custom-notes
memory_type: episodic
summary: Unknown category file.
max_lines: 80
---

# Custom Notes

- note one
`)
	writeTestFile(t, tmp, "no-category.md", `---
memory_type: semantic
summary: No category set.
max_lines: 80
---

# No Category

- item one
`)

	idx, err := memoryIndex(tmp)
	if err != nil {
		t.Fatalf("memoryIndex: %v", err)
	}

	aboutPos := strings.Index(idx, "about-me")
	customPos := strings.Index(idx, "custom-notes")
	noCatPos := strings.Index(idx, "no-category")

	if aboutPos < 0 {
		t.Error("index missing about-me")
	}
	if customPos < 0 {
		t.Error("index missing custom-notes (unknown category)")
	}
	if noCatPos < 0 {
		t.Error("index missing no-category (empty category, should use filename stem)")
	}
	if aboutPos > customPos {
		t.Error("canonical category about-me should appear before unknown category custom-notes")
	}
	if aboutPos > noCatPos {
		t.Error("canonical category about-me should appear before no-category entry")
	}
}
