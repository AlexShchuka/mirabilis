package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
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

func TestCommitEcosystemCommitsOnlyDirtyRepos(t *testing.T) {
	hdir := t.TempDir()
	t.Setenv("HOME", hdir)
	root := filepath.Join(hdir, ecosystemDirRel)

	dirty := filepath.Join(root, "darwin")
	clean := filepath.Join(root, "mirabilis")
	notRepo := filepath.Join(root, "loose")
	for _, d := range []string{dirty, clean, notRepo} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dirty, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(clean, ".git", "HEAD"), "ref: refs/heads/main\n")

	f := exec.NewFake()
	f.Expect([]string{"git", "-C", dirty, "rev-parse", "--git-dir"}, ".git", nil)
	f.Expect([]string{"git", "-C", dirty, "status", "--porcelain"}, " M file.txt\n", nil)
	f.Expect([]string{"git", "-C", dirty, "add", "-A"}, "", nil)
	f.Expect([]string{"git", "-C", dirty, "commit", "-m"}, "", nil)
	f.Expect([]string{"git", "-C", dirty, "push", "origin", "HEAD:main"}, "", nil)
	f.Expect([]string{"git", "-C", clean, "rev-parse", "--git-dir"}, ".git", nil)
	f.Expect([]string{"git", "-C", clean, "status", "--porcelain"}, "", nil)
	setRunner(t, f)

	committed, pushed := commitEcosystem(t.Context())
	if committed != 1 {
		t.Errorf("commitEcosystem committed %d repos, want 1 (dirty only)", committed)
	}
	if pushed != 1 {
		t.Errorf("commitEcosystem pushed %d repos, want 1 (the committed dirty repo)", pushed)
	}
	if rem := f.Remaining(); rem != 0 {
		t.Errorf("commitEcosystem left %d unused stubs", rem)
	}
}

func TestCommitEcosystemPushNonFastForwardSkipped(t *testing.T) {
	hdir := t.TempDir()
	t.Setenv("HOME", hdir)
	root := filepath.Join(hdir, ecosystemDirRel)

	dirty := filepath.Join(root, "darwin")
	if err := os.MkdirAll(dirty, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dirty, ".git", "HEAD"), "ref: refs/heads/main\n")

	f := exec.NewFake()
	f.Expect([]string{"git", "-C", dirty, "rev-parse", "--git-dir"}, ".git", nil)
	f.Expect([]string{"git", "-C", dirty, "status", "--porcelain"}, " M file.txt\n", nil)
	f.Expect([]string{"git", "-C", dirty, "add", "-A"}, "", nil)
	f.Expect([]string{"git", "-C", dirty, "commit", "-m"}, "", nil)
	f.Expect([]string{"git", "-C", dirty, "push", "origin", "HEAD:main"}, "",
		errors.New("! [rejected] HEAD -> main (non-fast-forward)"))
	setRunner(t, f)

	committed, pushed := commitEcosystem(t.Context())
	if committed != 1 {
		t.Errorf("commitEcosystem committed %d repos, want 1", committed)
	}
	if pushed != 0 {
		t.Errorf("commitEcosystem pushed %d repos, want 0 (rejected push must be skipped, not forced)", pushed)
	}
	if rem := f.Remaining(); rem != 0 {
		t.Errorf("commitEcosystem left %d unused stubs (no force-push retry expected)", rem)
	}
}

func TestCommitEcosystemNoRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	committed, pushed := commitEcosystem(t.Context())
	if committed != 0 || pushed != 0 {
		t.Errorf("commitEcosystem = (%d, %d) with no ecosystem dir, want (0, 0)", committed, pushed)
	}
}
