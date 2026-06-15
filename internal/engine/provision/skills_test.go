package provision

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsCheckTrueWhenGolangDirPresent(t *testing.T) {
	t.Parallel()
	d, _ := testDeps(t)
	mustWrite(t, d.Cfg.SkillsTxt(), ccSkillsGolang+"\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileSkills), ccSkillsGolang+"\n")
	golangDir := filepath.Join(d.claudeDir(), "skills", "golang-channels")
	if err := os.MkdirAll(golangDir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &skillsStep{d: d}
	if !checkStep(t, s) {
		t.Fatal("Check returned false but golang-* dir present (INV-GATEFREE violated)")
	}
}

func TestSkillsCheckFalseWhenGolangDirAbsent(t *testing.T) {
	t.Parallel()
	d, _ := testDeps(t)
	mustWrite(t, d.Cfg.SkillsTxt(), ccSkillsGolang+"\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileSkills), ccSkillsGolang+"\n")
	if err := os.MkdirAll(d.claudeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	s := &skillsStep{d: d}
	if checkStep(t, s) {
		t.Fatal("Check returned true but no golang-* dirs installed (INV-GATEFREE violated)")
	}
}

func TestSkillsInstallGolangIdempotent(t *testing.T) {
	t.Parallel()
	d, f := testDeps(t)
	mustWrite(t, d.Cfg.SkillsTxt(), ccSkillsGolang+"\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileSkills), ccSkillsGolang+"\n")
	golangDir := filepath.Join(d.claudeDir(), "skills", "golang-channels")
	if err := os.MkdirAll(golangDir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &skillsStep{d: d}
	if err := runStep(t, s); err != nil {
		t.Fatalf("Run returned error when skills already present (idempotency violated): %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("exec called %d time(s) when skills already present; want 0 (gh must not run again)", n)
	}
}

func TestSkillsInstallGolangGhArgv(t *testing.T) {
	t.Parallel()
	d, f := testDeps(t)
	mustWrite(t, d.Cfg.SkillsTxt(), ccSkillsGolang+"\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileSkills), ccSkillsGolang+"\n")
	if err := os.MkdirAll(filepath.Join(d.claudeDir(), "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := &skillsStep{d: d}
	skillsDir := s.skillsDir()
	expectedArgv := []string{"gh", "skill", "install", ccSkillsGolang, "--all", "-f", "--dir", skillsDir}
	f.Expect(expectedArgv, "", nil)
	if err := runStep(t, s); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("expected gh stub was not consumed: %d remaining", n)
	}
}

func TestSkillsGitEntryIdempotentWhenDirPresent(t *testing.T) {
	t.Parallel()
	d, f := testDeps(t)
	mustWrite(t, d.Cfg.SkillsTxt(), "owner/repo-a\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileSkills), "owner/repo-a\n")
	dir := filepath.Join(d.claudeDir(), "skills", "repo-a")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	gv := []string{"git", "version"}
	f.Expect(gv, "", nil)
	f.Expect([]string{"git", "-C", dir, "pull", "--ff-only"}, "", nil)
	s := &skillsStep{d: d}
	if err := runStep(t, s); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("unexpected unused stubs: %d", n)
	}
}
