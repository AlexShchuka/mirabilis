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

const checkTimeout = 30 * time.Second

const (
	harnessPrefScript = `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`
	harnessSkip       = "skip"
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
