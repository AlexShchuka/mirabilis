package steps

import (
	"context"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type imageStep struct {
	d Deps
}

func (s *imageStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    "image",
		Title:   "Image",
		Deps:    []string{"preflight", "stacks"},
		Kind:    pipeline.Auto,
		Timeout: 15 * time.Minute,
	}
}

func (s *imageStep) Check(ctx context.Context) (bool, error) {
	return !s.d.Sandbox.Stale(ctx), nil
}

func (s *imageStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	return stream("image", out, s.d.Sandbox.Build(ctx))
}
