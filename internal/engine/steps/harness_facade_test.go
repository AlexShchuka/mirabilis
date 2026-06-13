package steps

import (
	"errors"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

const (
	prefSkipScript    = `printf '%s\n' skip > "$HOME/.claude/.mirabilis-harness"`
	prefInstallScript = `printf '%s\n' install > "$HOME/.claude/.mirabilis-harness"`
)

func facadeDeps(t *testing.T, fake *exec.Fake) Deps {
	t.Helper()
	return newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())
}

func TestHarnessStatus(t *testing.T) {
	t.Parallel()
	t.Run("no claude returns container error", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(harnessBash("command -v claude"), "", errors.New("missing"))
		if _, err := HarnessStatus(t.Context(), facadeDeps(t, fake)); !errors.Is(err, errHarnessContainer) {
			t.Fatalf("err = %v, want errHarnessContainer", err)
		}
	})
	t.Run("skip preference is off", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(harnessPrefScript), "skip\n", nil)
		got, err := HarnessStatus(t.Context(), facadeDeps(t, fake))
		if err != nil || got != HarnessOff {
			t.Fatalf("status = %q, %v, want off", got, err)
		}
	})
	t.Run("not installed is missing", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(harnessPrefScript), "install\n", nil).
			Expect(harnessBash(harness.ProbeScript), "", errors.New("absent"))
		got, err := HarnessStatus(t.Context(), facadeDeps(t, fake))
		if err != nil || got != HarnessMissing {
			t.Fatalf("status = %q, %v, want missing", got, err)
		}
	})
	t.Run("disabled is off", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(harnessPrefScript), "install\n", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil).
			Expect(harnessBash(harnessDisabledScript), "", nil)
		got, err := HarnessStatus(t.Context(), facadeDeps(t, fake))
		if err != nil || got != HarnessOff {
			t.Fatalf("status = %q, %v, want off", got, err)
		}
	})
	t.Run("installed and enabled is on", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(harnessPrefScript), "install\n", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil).
			Expect(harnessBash(harnessDisabledScript), "", errors.New("not disabled"))
		got, err := HarnessStatus(t.Context(), facadeDeps(t, fake))
		if err != nil || got != HarnessOn {
			t.Fatalf("status = %q, %v, want on", got, err)
		}
	})
}

func TestHarnessApply(t *testing.T) {
	t.Parallel()
	t.Run("no claude returns container error", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(harnessBash("command -v claude"), "", errors.New("missing"))
		if err := HarnessApply(t.Context(), facadeDeps(t, fake), HarnessOn); !errors.Is(err, errHarnessContainer) {
			t.Fatalf("err = %v, want errHarnessContainer", err)
		}
	})
	t.Run("off disables when installed and enabled", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(prefSkipScript), "", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil).
			Expect(harnessBash(harnessDisabledScript), "", errors.New("not disabled")).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "disable", "neuro-matrix@neuro-matrix"}, "", nil)
		if err := HarnessApply(t.Context(), facadeDeps(t, fake), HarnessOff); err != nil {
			t.Fatalf("apply off: %v", err)
		}
		if fake.Remaining() != 0 {
			t.Fatalf("unused stubs: %d", fake.Remaining())
		}
	})
	t.Run("on installs when missing", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(prefInstallScript), "", nil).
			Expect(harnessBash(harness.ProbeScript), "", errors.New("absent")).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "marketplace", "add"}, "", nil).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "install"}, "", nil).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "update"}, "", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil).
			Expect(harnessBash(harness.RelinkScript), "", nil)
		if err := HarnessApply(t.Context(), facadeDeps(t, fake), HarnessOn); err != nil {
			t.Fatalf("apply on: %v", err)
		}
		if fake.Remaining() != 0 {
			t.Fatalf("unused stubs: %d", fake.Remaining())
		}
	})
	t.Run("on enables and relinks when disabled", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(prefInstallScript), "", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil).
			Expect(harnessBash(harnessDisabledScript), "", nil).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "enable", "neuro-matrix@neuro-matrix"}, "", nil).
			Expect(harnessBash(harness.RelinkScript), "", nil)
		if err := HarnessApply(t.Context(), facadeDeps(t, fake), HarnessOn); err != nil {
			t.Fatalf("apply on (disabled): %v", err)
		}
		if fake.Remaining() != 0 {
			t.Fatalf("unused stubs: %d", fake.Remaining())
		}
	})
	t.Run("reinstall uninstalls then installs", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().
			Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
			Expect(harnessBash(prefInstallScript), "", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "uninstall", "neuro-matrix@neuro-matrix"}, "", nil).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "marketplace", "add"}, "", nil).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "install"}, "", nil).
			Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "update"}, "", nil).
			Expect(harnessBash(harness.ProbeScript), "", nil).
			Expect(harnessBash(harness.RelinkScript), "", nil)
		if err := HarnessApply(t.Context(), facadeDeps(t, fake), HarnessReinstall); err != nil {
			t.Fatalf("apply reinstall: %v", err)
		}
		if fake.Remaining() != 0 {
			t.Fatalf("unused stubs: %d", fake.Remaining())
		}
	})
	t.Run("unknown choice errors", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil)
		if err := HarnessApply(t.Context(), facadeDeps(t, fake), "bogus"); err == nil {
			t.Fatal("apply bogus = nil, want unknown-choice error")
		}
	})
}

func TestWriteHarnessPref(t *testing.T) {
	t.Parallel()
	t.Run("writes value", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(harnessBash(prefSkipScript), "", nil)
		if err := writeHarnessPref(t.Context(), facadeDeps(t, fake), harnessSkip); err != nil {
			t.Fatalf("writeHarnessPref: %v", err)
		}
	})
	t.Run("propagates exec error", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect(harnessBash(prefSkipScript), "", errors.New("denied"))
		if err := writeHarnessPref(t.Context(), facadeDeps(t, fake), harnessSkip); err == nil {
			t.Fatal("writeHarnessPref = nil, want error")
		}
	})
}
