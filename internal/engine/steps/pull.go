package steps

import (
	"context"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

type pullImageStep struct {
	d     Deps
	image string
	name  string
}

func newPullBuild(d Deps) *pullImageStep {
	return &pullImageStep{d: d, image: sandbox.BaseImageBuild, name: "pull-build"}
}

func newPullRuntime(d Deps) *pullImageStep {
	return &pullImageStep{d: d, image: sandbox.BaseImageRuntime, name: "pull-runtime"}
}

func (s *pullImageStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     s.name,
		Title:    "Pull (" + s.image + ")",
		Deps:     []string{"preflight"},
		Kind:     pipeline.Auto,
		Parallel: true,
		Timeout:  15 * time.Minute,
	}
}

func (s *pullImageStep) Check(ctx context.Context) (bool, error) {
	return s.d.Sandbox.ImagePresent(ctx, s.image), nil
}

func (s *pullImageStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	return stream(s.name, out, s.d.Sandbox.PullImage(ctx, s.image))
}
