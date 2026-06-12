package steps

import (
	"context"
	"errors"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

type attachStep struct {
	d Deps
}

func (s *attachStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:  "attach",
		Title: "Claude",
		Deps:  []string{"claude-auth", "provision-start"},
		Kind:  pipeline.Terminal,
	}
}

func (s *attachStep) Check(context.Context) (bool, error) {
	return false, nil
}

func (s *attachStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	token, err := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: containerArgv("gh", "auth", "token")})
	token = strings.TrimSpace(token)
	if err != nil || token == "" {
		return errors.New("GitHub token is not available — sign in with gh auth login first")
	}
	argv := sandbox.BuildAttachArgv(s.d.Sandbox.SystemPromptFile(ctx))
	out <- pipeline.Event{
		Kind: pipeline.EvWaiting,
		Step: "attach",
		Argv: argv,
		Env:  []string{"GITHUB_PERSONAL_ACCESS_TOKEN=" + token},
	}
	_, err = awaitResume(ctx, in)
	return err
}
