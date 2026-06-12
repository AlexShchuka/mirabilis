package steps

import (
	"errors"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

const ghLoginTranscript = `Tip: you can generate a Personal Access Token here https://github.com/settings/tokens
! First copy your one-time code: 1B2C-D3E4
Open this URL to continue in your web browser: https://github.com/login/device.
Authentication complete.
`

func newGHAuthForTest(t *testing.T, fake exec.Runner) *ghAuthStep {
	t.Helper()
	return &ghAuthStep{d: newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())}
}

func TestGHAuthCheck(t *testing.T) {
	t.Parallel()
	status := []string{"docker", "exec", "mirabilis", "gh", "auth", "status"}
	t.Run("signed in", func(t *testing.T) {
		t.Parallel()
		mustCheck(t, newGHAuthForTest(t, exec.NewFake().Expect(status, "", nil)), true)
	})
	t.Run("signed out", func(t *testing.T) {
		t.Parallel()
		mustCheck(t, newGHAuthForTest(t, exec.NewFake().Expect(status, "", errors.New("not logged in"))), false)
	})
}

func TestGHAuthRunExtractsCodeAndURL(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().Expect([]string{"docker", "exec", "-i", "mirabilis", "env"}, ghLoginTranscript, nil)
	s := newGHAuthForTest(t, fake)
	evs, err := runStep(t, s, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got, ok := waitingEvent(t, evs).Payload.(GHAuth)
	if !ok {
		t.Fatalf("payload = %T, want GHAuth", waitingEvent(t, evs).Payload)
	}
	want := GHAuth{Code: "1B2C-D3E4", URL: "https://github.com/login/device"}
	if got != want {
		t.Fatalf("payload = %+v, want %+v", got, want)
	}
	waitings := 0
	lines := 0
	for _, ev := range evs {
		switch ev.Kind {
		case pipeline.EvWaiting:
			waitings++
		case pipeline.EvLine:
			lines++
		}
	}
	if waitings != 1 {
		t.Fatalf("EvWaiting emitted %d times, want 1", waitings)
	}
	if lines != 4 {
		t.Fatalf("forwarded %d lines, want 4", lines)
	}
}

func TestGHAuthRunLoginArgv(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().Expect([]string{"docker"}, "", nil)
	s := newGHAuthForTest(t, fake)
	if _, err := runStep(t, s, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{
		"docker", "exec", "-i", "mirabilis",
		"env", "BROWSER=true",
		"gh", "auth", "login", "--hostname", "github.com",
		"--git-protocol", "https", "--web", "--scopes", "workflow",
		"--insecure-storage",
	}
	got := fake.Calls()[0].Argv
	if len(got) != len(want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGHAuthRunFailurePropagates(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().Expect([]string{"docker", "exec", "-i"}, "", errors.New("login failed"))
	s := newGHAuthForTest(t, fake)
	if _, err := runStep(t, s, nil); err == nil {
		t.Fatal("want login failure")
	}
}

func TestGHAuthRunCancelled(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().ExpectHang([]string{"docker", "exec", "-i"})
	s := newGHAuthForTest(t, fake)
	out := make(chan pipeline.Event)
	in := make(chan pipeline.Result)
	go func() {
		for ev := range out {
			if ev.Kind == pipeline.EvSpawn {
				in <- pipeline.Result{Cancelled: true}
			}
		}
	}()
	err := s.Run(t.Context(), out, in)
	close(out)
	if !errors.Is(err, pipeline.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}
