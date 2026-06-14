package steps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	checkTimeout        = 30 * time.Second
	harnessApplyTimeout = 15 * time.Minute
)

const (
	harnessPrefScript     = `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`
	harnessDisabledScript = "claude plugin list 2>/dev/null | grep -A3 neuro-matrix | grep -qw disabled"
	harnessSkip           = "skip"
	harnessInstallPref    = "install"
)

const (
	HarnessOn        = "on"
	HarnessOff       = "off"
	HarnessMissing   = "missing"
	HarnessReinstall = "reinstall"
)

var errHarnessContainer = errors.New("steps: harness: container claude unavailable — run Launch first")

type harnessStep struct {
	d Deps
}

func (s *harnessStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     "harness",
		Title:    "Harness",
		Deps:     []string{"provision-start"},
		Kind:     pipeline.Auto,
		Optional: true,
		Timeout:  5 * time.Minute,
	}
}

func (s *harnessStep) Check(ctx context.Context) (bool, error) {
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	pref, _ := exec.Run(checkCtx, s.d.Runner, exec.Spec{Argv: containerArgv("bash", "-lc", harnessPrefScript)})
	if strings.TrimSpace(pref) == harnessSkip {
		return true, nil
	}
	return scriptOK(checkCtx, s.d, harness.ProbeScript), nil
}

func (s *harnessStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !scriptOK(ctx, s.d, "command -v claude") {
		return errHarnessContainer
	}
	return installHarness(ctx, s.d, out)
}

func scriptOK(ctx context.Context, d Deps, script string) bool {
	_, err := exec.Run(ctx, d.Runner, exec.Spec{Argv: containerArgv("bash", "-lc", script)})
	return err == nil
}

func streamHarness(ctx context.Context, d Deps, out chan<- pipeline.Event, args ...string) error {
	return stream("harness", out, d.Runner.Stream(ctx, exec.Spec{Argv: containerArgv(args...)}))
}

func installHarness(ctx context.Context, d Deps, out chan<- pipeline.Event) error {
	var errs []error
	for _, a := range harness.InstallActions() {
		err := streamHarness(ctx, d, out, a.Argv...)
		if err != nil && a.Fallback != nil {
			err = streamHarness(ctx, d, out, a.Fallback...)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", a.WrapErr, err))
		}
	}
	if !scriptOK(ctx, d, harness.ProbeScript) {
		errs = append(errs, errors.New("neuro-matrix not present after install"))
	}
	if err := streamHarness(ctx, d, out, "bash", "-lc", harness.RelinkScript); err != nil {
		errs = append(errs, fmt.Errorf("neuro-matrix symlink: %w", err))
	}
	return errors.Join(errs...)
}

func writeHarnessPref(ctx context.Context, d Deps, value string) error {
	script := `printf '%s\n' ` + value + ` > "$HOME/.claude/.mirabilis-harness"`
	if _, err := exec.Run(ctx, d.Runner, exec.Spec{Argv: containerArgv("bash", "-lc", script)}); err != nil {
		return fmt.Errorf("steps: harness: write preference: %w", err)
	}
	return nil
}

func HarnessStatus(ctx context.Context, d Deps) (string, error) {
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	if !scriptOK(checkCtx, d, "command -v claude") {
		return "", errHarnessContainer
	}
	pref, _ := exec.Run(checkCtx, d.Runner, exec.Spec{Argv: containerArgv("bash", "-lc", harnessPrefScript)})
	if strings.TrimSpace(pref) == harnessSkip {
		return HarnessOff, nil
	}
	if !scriptOK(checkCtx, d, harness.ProbeScript) {
		return HarnessMissing, nil
	}
	if scriptOK(checkCtx, d, harnessDisabledScript) {
		return HarnessOff, nil
	}
	return HarnessOn, nil
}

func HarnessApply(ctx context.Context, d Deps, choice string) error {
	ctx, cancel := context.WithTimeout(ctx, harnessApplyTimeout)
	defer cancel()
	if !scriptOK(ctx, d, "command -v claude") {
		return errHarnessContainer
	}

	out := make(chan pipeline.Event, 64)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range out {
		}
	}()
	defer func() {
		close(out)
		<-drained
	}()

	switch choice {
	case HarnessOff:
		if err := writeHarnessPref(ctx, d, harnessSkip); err != nil {
			return err
		}
		if scriptOK(ctx, d, harness.ProbeScript) && !scriptOK(ctx, d, harnessDisabledScript) {
			return streamHarness(ctx, d, out, "claude", "plugin", "disable", "neuro-matrix@neuro-matrix")
		}
		return nil
	case HarnessOn:
		if err := writeHarnessPref(ctx, d, harnessInstallPref); err != nil {
			return err
		}
		if !scriptOK(ctx, d, harness.ProbeScript) {
			return installHarness(ctx, d, out)
		}
		if scriptOK(ctx, d, harnessDisabledScript) {
			if err := streamHarness(ctx, d, out, "claude", "plugin", "enable", "neuro-matrix@neuro-matrix"); err != nil {
				return err
			}
		}
		return streamHarness(ctx, d, out, "bash", "-lc", harness.RelinkScript)
	case HarnessReinstall:
		if err := writeHarnessPref(ctx, d, harnessInstallPref); err != nil {
			return err
		}
		if scriptOK(ctx, d, harness.ProbeScript) {
			if err := streamHarness(ctx, d, out, "claude", "plugin", "uninstall", "neuro-matrix@neuro-matrix"); err != nil {
				return fmt.Errorf("steps: harness: uninstall before reinstall: %w", err)
			}
		}
		return installHarness(ctx, d, out)
	}
	return fmt.Errorf("steps: harness: unknown choice %q", choice)
}
