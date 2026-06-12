package steps

import (
	"context"
	"errors"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

type containerStep struct {
	d    Deps
	poll time.Duration
	wait time.Duration
}

func newContainer(d Deps) *containerStep {
	return &containerStep{d: d, poll: 2 * time.Second, wait: 120 * time.Second}
}

func (s *containerStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    "container",
		Title:   "Container",
		Deps:    []string{"image"},
		Kind:    pipeline.Auto,
		Timeout: 3 * time.Minute,
		Retry:   pipeline.RetryPolicy{Attempts: 2, Delay: 2 * time.Second},
	}
}

func healthy(c sandbox.Container) bool {
	return c.Health == "" || c.Health == "healthy"
}

func (s *containerStep) Check(ctx context.Context) (bool, error) {
	c, err := s.d.Docker.Inspect(ctx)
	if err != nil || !c.Running || !healthy(c) {
		return false, nil
	}
	return c.Env["MIRABILIS_VERSION"] == s.d.Sandbox.Desired(ctx), nil
}

func (s *containerStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if c, err := s.d.Docker.Inspect(ctx); err == nil && c.Running && c.Env["MIRABILIS_VERSION"] != s.d.Sandbox.Desired(ctx) {
		if err := stream("container", out, s.d.Sandbox.Down(ctx)); err != nil {
			return err
		}
	}
	if err := stream("container", out, s.d.Sandbox.Up(ctx)); err != nil {
		return err
	}
	return s.awaitHealthy(ctx)
}

func (s *containerStep) awaitHealthy(ctx context.Context) error {
	deadline := time.Now().Add(s.wait)
	for {
		if c, err := s.d.Docker.Inspect(ctx); err == nil && c.Running && healthy(c) {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("container did not become healthy")
		}
		select {
		case <-time.After(s.poll):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
