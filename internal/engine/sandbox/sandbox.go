package sandbox

import (
	"context"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const (
	composeFile     = "docker-compose.yml"
	composeSockFile = "compose.sock.yml"

	BaseImageBuild   = "golang:1.26-trixie@sha256:bbf22ddccb3205344f2755ea8fa4fe39f7a8b2b77b9f7b764ec2aad31406f6fc"
	BaseImageRuntime = "node:26-trixie-slim@sha256:191ef878ecb351d68b78219593de18bd8942afd59af59f29960dc4b24805a3f1"
)

type Sandbox struct {
	runner exec.Runner
	docker Docker
	repo   string
}

func New(runner exec.Runner, docker Docker, repo string) *Sandbox {
	return &Sandbox{runner: runner, docker: docker, repo: repo}
}

func (s *Sandbox) Build(ctx context.Context) <-chan exec.Event {
	return s.compose(ctx, "build")
}

func (s *Sandbox) PullImage(ctx context.Context, image string) <-chan exec.Event {
	return s.runner.Stream(ctx, exec.Spec{Argv: []string{"docker", "pull", image}})
}

func (s *Sandbox) ImagePresent(ctx context.Context, image string) bool {
	_, err := exec.Run(ctx, s.runner, exec.Spec{Argv: []string{"docker", "image", "inspect", image}})
	return err == nil
}

func (s *Sandbox) Up(ctx context.Context) <-chan exec.Event {
	return s.compose(ctx, "up", "-d")
}

func (s *Sandbox) Down(ctx context.Context) <-chan exec.Event {
	return s.compose(ctx, "down")
}

func (s *Sandbox) compose(ctx context.Context, args ...string) <-chan exec.Event {
	argv := []string{"docker", "compose", "-f", composeFile}
	if config.Sock(s.repo) {
		argv = append(argv, "-f", composeSockFile)
	}
	argv = append(argv, args...)
	return s.runner.Stream(ctx, exec.Spec{
		Argv: argv,
		Dir:  s.repo,
		Env:  s.composeEnv(ctx),
	})
}

func (s *Sandbox) composeEnv(ctx context.Context) []string {
	env := []string{"MIRABILIS_VERSION=" + s.Desired(ctx)}
	if stacks, ok := config.ReadStacks(s.repo); ok {
		env = append(env, "STACKS="+stacks)
	}
	if tz := os.Getenv("TZ"); tz != "" {
		env = append(env, "TZ="+tz)
	}
	return env
}

func (s *Sandbox) Desired(ctx context.Context) string {
	sha, err := exec.Run(ctx, s.runner, exec.Spec{
		Argv: []string{"git", "-C", s.repo, "rev-parse", "--short", "HEAD"},
		Dir:  s.repo,
	})
	sha = strings.TrimSpace(sha)
	if err != nil || sha == "" {
		sha = "unknown"
	}
	stacks, _ := config.ReadStacks(s.repo)
	fp := sha + "-" + stacks
	if config.Sock(s.repo) {
		fp += "-sock"
	}
	return fp
}

func (s *Sandbox) Running(ctx context.Context) string {
	c, err := s.docker.Inspect(ctx)
	if err != nil || !c.Running {
		return ""
	}
	return c.Env["MIRABILIS_VERSION"]
}

func (s *Sandbox) Stale(ctx context.Context) bool {
	running := s.Running(ctx)
	return running == "" || running != s.Desired(ctx)
}

func (s *Sandbox) WillRecreate(ctx context.Context) bool {
	return s.Running(ctx) != "" && s.Stale(ctx)
}

func drain(events <-chan exec.Event) error {
	var err error
	for ev := range events {
		if ev.Kind == exec.KindExited {
			err = ev.Err
		}
	}
	return err
}
