package auth

import (
	"context"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type step struct{}

func (step) Check(ctx context.Context, r runner.Runner) (bool, error) {
	_, err := r.Container(ctx, "gh", "auth", "status")
	return err == nil, nil
}

func (step) Run(_ context.Context, _ runner.Runner) error {
	return nil
}

func Steps() []pipeline.Registered {
	return []pipeline.Registered{
		{
			Meta: pipeline.StepMeta{
				Name:        "gh",
				Title:       "GitHub sign-in",
				Detail:      "checking GitHub sign-in",
				Deps:        []string{"prepare"},
				Optional:    false,
				Interactive: true,
				Timeout:     30 * time.Second,
			},
			Impl: step{},
		},
	}
}
