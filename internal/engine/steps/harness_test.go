package steps

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func harnessBash(script string) []string {
	return []string{"docker", "exec", "mirabilis", "bash", "-lc", script}
}

func newHarnessForTest(t *testing.T, fake *exec.Fake) *harnessStep {
	t.Helper()
	return &harnessStep{d: newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())}
}

func TestHarnessCheck(t *testing.T) {
	t.Parallel()
	t.Run("skip preference", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(harnessBash(harnessPrefScript), "skip\n", nil)
		mustCheck(t, newHarnessForTest(t, fake), true)
	})
	t.Run("installed", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash(harnessPrefScript), "", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil)
		mustCheck(t, newHarnessForTest(t, fake), true)
	})
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash(harnessPrefScript), "", nil).
			Expect(harnessBash(harness.ProbeScript), "", errors.New("not found"))
		mustCheck(t, newHarnessForTest(t, fake), false)
	})
}

func TestHarnessRunInstalls(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
		Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "marketplace", "add"}, "", nil).
		Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "install"}, "", nil).
		Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "update"}, "", nil).
		Expect(harnessBash(harness.ProbeScript), "", nil).
		Expect(harnessBash(harness.RelinkScript), "", nil)
	s := newHarnessForTest(t, fake)
	if _, err := runStep(t, s, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if fake.Remaining() != 0 {
		t.Fatalf("unused stubs: %d", fake.Remaining())
	}
}

func TestHarnessRunFallsBackToMarketplaceUpdate(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
		Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "marketplace", "add"}, "", errors.New("exists")).
		Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "marketplace", "update"}, "", nil).
		Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "install"}, "", nil).
		Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "update"}, "", nil).
		Expect(harnessBash(harness.ProbeScript), "", nil).
		Expect(harnessBash(harness.RelinkScript), "", nil)
	s := newHarnessForTest(t, fake)
	if _, err := runStep(t, s, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"docker", "exec", "mirabilis", "claude", "plugin", "marketplace", "update", "neuro-matrix"}
	if got := fake.Calls()[2].Argv; !slices.Equal(got, want) {
		t.Fatalf("fallback argv = %v, want %v", got, want)
	}
}

func TestHarnessCheckHonorsDeadline(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect(harnessBash(harnessPrefScript), "", nil).
		ExpectHang(harnessBash(harness.ProbeScript))
	s := newHarnessForTest(t, fake)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	ok, err := s.Check(ctx)
	elapsed := time.Since(start)
	if ok {
		t.Fatal("Check = true, want false on hung exec")
	}
	if err != nil {
		t.Fatalf("Check err = %v, want nil", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("Check took %v, want < 3s when parent ctx expires", elapsed)
	}
}

func TestHarnessRunFailsWithoutClaude(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect(harnessBash("command -v claude"), "", errors.New("not found"))
	s := newHarnessForTest(t, fake)
	if _, err := runStep(t, s, nil); !errors.Is(err, errHarnessContainer) {
		t.Fatalf("run err = %v, want errHarnessContainer", err)
	}
	if got := len(fake.Calls()); got != 1 {
		t.Fatalf("got %d calls, want 1", got)
	}
}
