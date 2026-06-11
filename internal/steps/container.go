package steps

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
)

type updateStep struct{}

func (updateStep) Check(ctx context.Context, r runner.Runner) (bool, error) {
	_, _ = r.Host(ctx, "git", "-C", r.Repo(), "fetch", "-q", "origin")
	out, err := r.Host(ctx, "git", "-C", r.Repo(), "rev-list", "--count", "HEAD..origin/main")
	if err != nil {
		return true, nil
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n == 0, nil
}

func (updateStep) Run(ctx context.Context, r runner.Runner) error {
	branch, _ := r.Host(ctx, "git", "-C", r.Repo(), "rev-parse", "--abbrev-ref", "HEAD")
	if strings.TrimSpace(branch) != "main" {
		return nil
	}
	if dirty, _ := r.Host(ctx, "git", "-C", r.Repo(), "status", "--porcelain"); dirty != "" {
		return fmt.Errorf("local changes present — commit or stash before updating")
	}
	_, err := r.Host(ctx, "git", "-C", r.Repo(), "merge", "--ff-only", "origin/main")
	return err
}

type prepareStep struct{}

func (prepareStep) Check(ctx context.Context, r runner.Runner) (bool, error) {
	return runtime.ContainerRunning(ctx, r) && !runtime.IsStale(ctx, r), nil
}

func (prepareStep) Run(ctx context.Context, r runner.Runner) error {
	repo := r.Repo()
	makeCmd := exec.CommandContext(ctx, "make", "-C", repo, "linux")
	makeCmd.Env = runtime.ComposeEnv(repo)
	if out, err := makeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("make linux failed: %s", runtime.LastLines(string(out), 12))
	}
	return pipeline.Retry(ctx, pipeline.RetryNet, func() error {
		up := exec.CommandContext(ctx, "devcontainer", "up", "--workspace-folder", repo)
		up.Env = runtime.ComposeEnv(repo)
		if out, err := up.CombinedOutput(); err != nil {
			return fmt.Errorf("devcontainer up failed: %s", runtime.LastLines(string(out), 12))
		}
		return nil
	})
}

func containerSteps() []pipeline.Registered {
	return []pipeline.Registered{
		{
			Meta: pipeline.StepMeta{
				Name:     "update",
				Title:    "Update (origin/main)",
				Detail:   "checking origin/main for updates",
				Retry:    pipeline.RetryNet,
				Optional: true,
				Timeout:  60 * time.Second,
			},
			Impl: updateStep{},
		},
		{
			Meta: pipeline.StepMeta{
				Name:   "prepare",
				Title:  "Container",
				Detail: "starting the container — the first image build may take a few minutes",
				Deps:   []string{"update"},
				Retry:  pipeline.RetryNone,
			},
			Impl: prepareStep{},
		},
	}
}
