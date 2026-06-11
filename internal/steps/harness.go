package steps

import (
	"context"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type harnessStep struct{}

func (harnessStep) Check(ctx context.Context, r runner.Runner) (bool, error) {
	return provision.HarnessInstalled(ctx, r)
}

func (harnessStep) Run(ctx context.Context, r runner.Runner) error {
	return provision.EnsureHarness(ctx, r)
}

func harnessSteps() []pipeline.Registered {
	return []pipeline.Registered{
		{
			Meta: pipeline.StepMeta{
				Name:     "harness",
				Title:    "neuro-matrix",
				Detail:   "installing/updating the neuro-matrix harness (network)",
				Deps:     []string{"prepare"},
				Retry:    pipeline.RetryNet,
				Optional: true,
				Timeout:  180 * time.Second,
			},
			Impl: harnessStep{},
		},
	}
}
