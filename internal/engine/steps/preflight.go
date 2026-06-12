package steps

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type preflightStep struct {
	d        Deps
	goos     string
	poll     time.Duration
	maxPolls int
}

func newPreflight(d Deps) *preflightStep {
	return &preflightStep{d: d, goos: runtime.GOOS, poll: 2 * time.Second, maxPolls: 30}
}

func (s *preflightStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    "preflight",
		Title:   "Preflight",
		Kind:    pipeline.Auto,
		Timeout: 90 * time.Second,
	}
}

func (s *preflightStep) Check(ctx context.Context) (bool, error) {
	if !s.dockerUp(ctx) {
		return false, nil
	}
	_, err := exec.Run(ctx, s.d.Runner, s.composeConfigSpec())
	return err == nil, nil
}

func (s *preflightStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.dockerUp(ctx) {
		if err := s.startDocker(ctx, out); err != nil {
			return err
		}
	}
	if err := stream("preflight", out, s.d.Runner.Stream(ctx, s.composeConfigSpec())); err != nil {
		return fmt.Errorf("compose file invalid: %w", err)
	}
	return nil
}

func (s *preflightStep) dockerUp(ctx context.Context) bool {
	_, err := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: []string{"docker", "version"}})
	return err == nil
}

func (s *preflightStep) composeConfigSpec() exec.Spec {
	return exec.Spec{
		Argv: []string{"docker", "compose", "-f", "docker-compose.yml", "config", "-q"},
		Dir:  s.d.Repo,
	}
}

func (s *preflightStep) startDocker(ctx context.Context, out chan<- pipeline.Event) error {
	if s.goos != "darwin" {
		return errors.New("docker is not running — start it and run mirabilis again")
	}
	_ = stream("preflight", out, s.d.Runner.Stream(ctx, exec.Spec{Argv: []string{"open", "-a", "Docker"}}))
	for i := 0; i < s.maxPolls; i++ {
		if s.dockerUp(ctx) {
			return nil
		}
		select {
		case <-time.After(s.poll):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errors.New("docker did not come up — open Docker Desktop and run mirabilis again")
}
