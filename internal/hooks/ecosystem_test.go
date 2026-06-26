package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEcosystemContextEmptyWhenAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := ecosystemContext(); got != "" {
		t.Errorf("ecosystemContext = %q, want empty when no repos present", got)
	}
}

func TestEcosystemContextReadsDarwinIndexAndShieldMemory(t *testing.T) {
	hdir := t.TempDir()
	t.Setenv("HOME", hdir)

	writeFile(t, filepath.Join(hdir, ecosystemDirRel, "darwin", "ecosystem", "MEMORY-index.md"),
		"# Darwin memory\n\n- node alpha\n- node beta\n")
	writeFile(t, filepath.Join(hdir, ecosystemDirRel, "SolitaryEquilibriumShield", "memory", "b.md"),
		"# Bravo\n\nsecond bravo line\n")
	writeFile(t, filepath.Join(hdir, ecosystemDirRel, "SolitaryEquilibriumShield", "memory", "a.md"),
		"# Alpha\n\nfirst alpha line\nmore alpha\n")

	got := ecosystemContext()
	for _, want := range []string{
		"Ecosystem memory (darwin)",
		"node alpha",
		"Ecosystem memory (SolitaryEquilibriumShield)",
		"memory/a.md",
		"first alpha line",
		"memory/b.md",
		"second bravo line",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("context missing %q in:\n%s", want, got)
		}
	}
	if strings.Index(got, "memory/a.md") > strings.Index(got, "memory/b.md") {
		t.Error("shield memory files should be listed in sorted order (a before b)")
	}
}

func TestJoinContextDropsEmpty(t *testing.T) {
	got := joinContext("", "one", "", "two")
	if got != "one\ntwo" {
		t.Errorf("joinContext = %q, want \"one\\ntwo\"", got)
	}
}
