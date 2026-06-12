package steps

import (
	"errors"
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestPluginsApplyCheck(t *testing.T) {
	t.Parallel()
	cat := []string{"docker", "exec", "mirabilis", "bash", "-lc", `cat "$HOME/.claude/.mirabilis-plugins-disabled" 2>/dev/null`}
	t.Run("in sync", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(cat, "beta\nalpha\n", nil)
		d := newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
		if err := config.WritePluginsDisabled(d.Repo, []string{"alpha", "beta"}); err != nil {
			t.Fatal(err)
		}
		mustCheck(t, newPluginsApply(d), true)
	})
	t.Run("out of sync", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(cat, "", nil)
		d := newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
		if err := config.WritePluginsDisabled(d.Repo, []string{"alpha"}); err != nil {
			t.Fatal(err)
		}
		mustCheck(t, newPluginsApply(d), false)
	})
}

func TestPluginsApplyRun(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "env"}, "", nil).
		Expect([]string{"docker", "exec", "mirabilis", "mirabilis"}, "", nil)
	d := newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
	if err := config.WritePluginsDisabled(d.Repo, []string{"alpha", "beta"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runStep(t, newPluginsApply(d), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	calls := fake.Calls()
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	wantWrite := []string{
		"docker", "exec", "mirabilis",
		"env", "MDIS=alpha\nbeta", "bash", "-lc",
		`printf '%s' "$MDIS" > "$HOME/.claude/.mirabilis-plugins-disabled"`,
	}
	if !slices.Equal(calls[0].Argv, wantWrite) {
		t.Fatalf("write argv = %v, want %v", calls[0].Argv, wantWrite)
	}
	wantApply := []string{"docker", "exec", "mirabilis", "mirabilis", "provision", "--phase", "plugins"}
	if !slices.Equal(calls[1].Argv, wantApply) {
		t.Fatalf("apply argv = %v, want %v", calls[1].Argv, wantApply)
	}
}

func TestSkillsApplyCheckAndRun(t *testing.T) {
	t.Parallel()
	cat := []string{"docker", "exec", "mirabilis", "bash", "-lc", `cat "$HOME/.claude/.mirabilis-skills" 2>/dev/null`}
	fake := exec.NewFake().
		Expect(cat, "stale\n", nil).
		Expect([]string{"docker", "exec", "mirabilis", "env"}, "", nil).
		Expect([]string{"docker", "exec", "mirabilis", "mirabilis"}, "", nil)
	d := newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
	if err := config.WriteSkills(d.Repo, "writer,researcher"); err != nil {
		t.Fatal(err)
	}
	s := newSkillsApply(d)
	mustCheck(t, s, false)
	if _, err := runStep(t, s, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	calls := fake.Calls()
	wantWrite := []string{
		"docker", "exec", "mirabilis",
		"env", "MSKILLS=writer\nresearcher", "bash", "-lc",
		`printf '%s' "$MSKILLS" > "$HOME/.claude/.mirabilis-skills"`,
	}
	if !slices.Equal(calls[1].Argv, wantWrite) {
		t.Fatalf("write argv = %v, want %v", calls[1].Argv, wantWrite)
	}
	wantApply := []string{"docker", "exec", "mirabilis", "mirabilis", "provision", "--phase", "skills"}
	if !slices.Equal(calls[2].Argv, wantApply) {
		t.Fatalf("apply argv = %v, want %v", calls[2].Argv, wantApply)
	}
}

func TestApplyRunWriteFailureStops(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "env"}, "", errors.New("write failed"))
	d := newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
	if _, err := runStep(t, newPluginsApply(d), nil); err == nil {
		t.Fatal("want write failure")
	}
	if got := len(fake.Calls()); got != 1 {
		t.Fatalf("got %d calls, want 1 (no provision after failed write)", got)
	}
}
