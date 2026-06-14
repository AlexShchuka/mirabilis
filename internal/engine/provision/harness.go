package provision

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type harnessStep struct {
	d Deps
}

func (s *harnessStep) Meta() pipeline.Meta { return installMeta("harness", "neuro-matrix harness") }

func (s *harnessStep) installed(ctx context.Context) bool {
	if s.d.harnessChoice() == harnessSkip {
		return true
	}
	return s.d.scriptOK(ctx, harness.ProbeScript)
}

func (s *harnessStep) Check(ctx context.Context) (bool, error) {
	return s.installed(ctx), nil
}

func (s *harnessStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	var errs []error
	if !s.installed(ctx) {
		errs = append(errs, s.install(ctx, out))
	}
	if err := s.d.streamScript(ctx, "harness", out, harness.RelinkScript); err != nil {
		errs = append(errs, fmt.Errorf("neuro-matrix symlink: %w", err))
	}
	return errors.Join(errs...)
}

func (s *harnessStep) install(ctx context.Context, out chan<- pipeline.Event) error {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return nil
	}
	var errs []error
	for _, a := range harness.InstallActions() {
		err := s.d.stream(ctx, "harness", out, a.Argv...)
		if err != nil && a.Fallback != nil {
			err = s.d.stream(ctx, "harness", out, a.Fallback...)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", a.WrapErr, err))
		}
	}
	if !s.d.scriptOK(ctx, harness.ProbeScript) {
		errs = append(errs, errors.New("neuro-matrix not installed after reinstall"))
	}
	return errors.Join(errs...)
}
