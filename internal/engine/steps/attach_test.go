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

func TestAttachCheckAlwaysFalse(t *testing.T) {
	t.Parallel()
	s := &attachStep{d: newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore())}
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
