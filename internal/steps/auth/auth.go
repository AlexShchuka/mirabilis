package auth

import (
	"context"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/steps"
)

type step struct{}

func (step) Check(ctx context.Context, r runner.Runner) (bool, error) {
	_, err := r.Container(ctx, "gh", "auth", "status")
	return err == nil, nil
}

func (step) Run(_ context.Context, _ runner.Runner) error {
	return nil
}

func init() {
	steps.Register(pipeline.StepMeta{
		Name:        "gh",
		Title:       "GitHub sign-in",
		Detail:      "checking GitHub sign-in",
		Deps:        []string{"prepare"},
		Optional:    true,
		Interactive: true,
		Timeout:     30 * time.Second,
	}, step{})
}
