package steps

import (
	"context"

	"github.com/AlexShchuka/mirabilis/internal/engine/claudeauth"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type claudeAuthStep struct {
	d Deps
}

func (s *claudeAuthStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:  "claude-auth",
		Title: "Claude auth",
		Deps:  []string{"preflight"},
		Kind:  pipeline.Terminal,
	}
}

func (s *claudeAuthStep) Check(ctx context.Context) (bool, error) {
	return claudeauth.Present(ctx, s.d.Tokens), nil
}

func (s *claudeAuthStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: "claude-auth", Argv: claudeauth.SetupArgv()}
	_, err := awaitResume(ctx, in)
	return err
}
