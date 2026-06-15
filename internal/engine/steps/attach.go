package steps

import (
	"context"
	"errors"
	"fmt"
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
		Kind:  pipeline.Handoff,
	}
}

// Check is never a skip-gate for a hand-off: the pipeline runs it unconditionally
// because Kind=Handoff. The idempotency contract exempts it (see pipeline/contract.go).
func (s *attachStep) Check(context.Context) (bool, error) {
	return false, nil
}

func (s *attachStep) preflight(ctx context.Context) error {
	c, err := s.d.Docker.Inspect(ctx)
	if err != nil {
		return fmt.Errorf("attach: sandbox container is not running: %w", err)
	}
	if !c.Running {
		return errors.New("attach: sandbox container is not running")
	}
	if _, err := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: containerArgv("claude", "--version")}); err != nil {
		return fmt.Errorf("attach: claude is not installed in the sandbox container: %w", err)
	}
	return nil
}

func (s *attachStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	if err := s.preflight(ctx); err != nil {
		return err
	}
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
