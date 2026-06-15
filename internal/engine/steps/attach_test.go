package steps

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestAttachCheckFalseWhenNotRunning(t *testing.T) {
	t.Parallel()
	fakeDocker := sandbox.NewFakeDocker().StubInspect(sandbox.Container{Running: false}, nil)
	s := &attachStep{d: newTestDeps(t, exec.NewFake(), fakeDocker, newFakeStore())}
	mustCheck(t, s, false)
}

func TestAttachCheckTrueWhenRunningAndClaudeAccessible(t *testing.T) {
	t.Parallel()
	fakeDocker := sandbox.NewFakeDocker().StubInspect(sandbox.Container{Running: true}, nil)
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "claude", "--version"}, "claude 1.0", nil)
	s := &attachStep{d: newTestDeps(t, fake, fakeDocker, newFakeStore())}
	mustCheck(t, s, true)
}

func TestAttachCheckFalseWhenClaudeNotAccessible(t *testing.T) {
	t.Parallel()
	fakeDocker := sandbox.NewFakeDocker().StubInspect(sandbox.Container{Running: true}, nil)
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "claude", "--version"}, "", errors.New("not found"))
	s := &attachStep{d: newTestDeps(t, fake, fakeDocker, newFakeStore())}
	mustCheck(t, s, false)
}

func TestAttachRunEmitsAttachArgv(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "gh", "auth", "token"}, "gho_secret\n", nil).
		Expect([]string{"docker", "exec", "mirabilis", "bash", "-lc"}, "/tmp/mirabilis-system-prompt.md", nil)
	s := &attachStep{d: newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())}
	evs, err := runStep(t, s, func(any) pipeline.Result { return pipeline.Result{} })
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantArgv := sandbox.BuildAttachArgv("/tmp/mirabilis-system-prompt.md")
	ev := waitingEvent(t, evs)
	if got := ev.Argv; !slices.Equal(got, wantArgv) {
		t.Fatalf("waiting argv = %v, want %v", got, wantArgv)
	}
	for _, a := range ev.Argv {
		if strings.Contains(a, "gho_secret") {
			t.Errorf("token leaked into argv element: %q", a)
		}
	}
	found := false
	for _, e := range ev.Env {
		if e == "GITHUB_PERSONAL_ACCESS_TOKEN=gho_secret" {
			found = true
		}
	}
	if !found {
		t.Errorf("GITHUB_PERSONAL_ACCESS_TOKEN not found in Event.Env: %v", ev.Env)
	}
}

func TestAttachRunWithoutGHToken(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "gh", "auth", "token"}, "", errors.New("not logged in"))
	s := &attachStep{d: newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())}
	if _, err := runStep(t, s, nil); err == nil {
		t.Fatal("want missing-token error")
	}
}

func TestAttachExecSharedHelper(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "gh", "auth", "token"}, "gho_secret\n", nil).
		Expect([]string{"docker", "exec", "mirabilis", "bash", "-lc"}, "/tmp/p.md", nil)
	d := newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
	argv, env, err := AttachExec(t.Context(), d)
	if err != nil {
		t.Fatalf("AttachExec: %v", err)
	}
	want := sandbox.BuildAttachArgv("/tmp/p.md")
	if !slices.Equal(argv, want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
	for _, a := range argv {
		if strings.Contains(a, "gho_secret") {
			t.Errorf("token leaked into argv: %q", a)
		}
	}
	if len(env) != 1 || env[0] != "GITHUB_PERSONAL_ACCESS_TOKEN=gho_secret" {
		t.Fatalf("env = %v, want single GITHUB_PERSONAL_ACCESS_TOKEN entry", env)
	}
}

func TestAttachExecNoToken(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "gh", "auth", "token"}, "", errors.New("not logged in"))
	d := newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
	if _, _, err := AttachExec(t.Context(), d); err == nil {
		t.Fatal("AttachExec without token = nil error, want error")
	}
}

func TestAttachRunCancelled(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "exec", "mirabilis", "gh", "auth", "token"}, "gho_secret", nil).
		Expect([]string{"docker", "exec", "mirabilis", "bash", "-lc"}, "/tmp/p.md", nil)
	s := &attachStep{d: newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())}
	_, err := runStep(t, s, func(any) pipeline.Result { return pipeline.Result{Cancelled: true} })
	if !errors.Is(err, pipeline.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}
