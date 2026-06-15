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
	d    Deps
	deps []string
}

func (s *attachStep) Meta() pipeline.Meta {
	deps := s.deps
	if len(deps) == 0 {
		deps = []string{"claude-auth", "provision-start"}
	}
	return pipeline.Meta{
		Name:  "attach",
		Title: "Claude",
		Deps:  deps,
		Kind:  pipeline.Terminal,
	}
}

func (s *attachStep) Check(ctx context.Context) (bool, error) {
	c, err := s.d.Docker.Inspect(ctx)
	if err != nil || !c.Running {
		return false, nil
	}
	_, err = exec.Run(ctx, s.d.Runner, exec.Spec{Argv: containerArgv("claude", "--version")})
	return err == nil, nil
}

func (s *attachStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	argv, env, err := AttachExec(ctx, s.d)
	if err != nil {
		return err
	}
	out <- pipeline.Event{
		Kind: pipeline.EvWaiting,
		Step: "attach",
		Argv: argv,
		Env:  env,
	}
	_, err = awaitResume(ctx, in)
	return err
}

func AttachExec(ctx context.Context, d Deps) (argv, env []string, err error) {
	token, terr := exec.Run(ctx, d.Runner, exec.Spec{Argv: containerArgv("gh", "auth", "token")})
	token = strings.TrimSpace(token)
	if terr != nil || token == "" {
		return nil, nil, errors.New("GitHub token is not available — sign in with gh auth login first")
	}
	argv = sandbox.BuildAttachArgv(d.Sandbox.SystemPromptFile(ctx))
	env = []string{"GITHUB_PERSONAL_ACCESS_TOKEN=" + token}
	return argv, env, nil
}
