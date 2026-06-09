package preflight

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/steps"
)

type step struct{}

func (step) Check(context.Context, runner.Runner) (bool, error) { return false, nil }

func (step) Run(ctx context.Context, r runner.Runner) error {
	if ip, _ := r.Container(ctx, "curl", "-s", "-m", "8", "https://api.ipify.org"); strings.TrimSpace(ip) == "" {
		return fmt.Errorf("egress: the container has no outbound network")
	}
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

func init() {
	steps.Register(pipeline.StepMeta{
		Name:    "preflight",
		Title:   "Environment check",
		Detail:  "checking egress to api.anthropic.com",
		Deps:    []string{"prepare", "harness", "gh"},
		Retry:   pipeline.RetryNone,
		Timeout: 60 * time.Second,
	}, step{})
}
