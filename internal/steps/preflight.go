package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type preflightStep struct{}

func (preflightStep) Check(context.Context, runner.Runner) (bool, error) { return false, nil }

func (preflightStep) Run(ctx context.Context, r runner.Runner) error {
	code, _ := r.Container(ctx, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-m", "12", "https://api.anthropic.com/v1/models")
	switch strings.TrimSpace(code) {
	case "200", "401", "403":
		return nil
	case "", "000":
		return fmt.Errorf("api.anthropic.com: unreachable")
	default:
		return fmt.Errorf("api.anthropic.com: HTTP %s", strings.TrimSpace(code))
	}
}

func preflightSteps() []pipeline.Registered {
	return []pipeline.Registered{
		{
			Meta: pipeline.StepMeta{
				Name:    "preflight",
				Title:   "Environment check",
				Detail:  "checking egress to api.anthropic.com",
				Deps:    []string{"prepare", "harness", "gh"},
				Retry:   pipeline.RetryNone,
				Timeout: 60 * time.Second,
			},
			Impl: preflightStep{},
		},
	}
}
