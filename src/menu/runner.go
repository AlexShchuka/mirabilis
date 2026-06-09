package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Runner interface {
	Host(ctx context.Context, name string, args ...string) (string, error)
	Container(ctx context.Context, args ...string) (string, error)
	Repo() string
}

type execRunner struct{ repo string }

func repoRoot() string {
	if r := os.Getenv("MIRABILIS_REPO"); r != "" {
		return r
	}
	exe, err := os.Executable()
	if err == nil {
		if resolved, e := filepath.EvalSymlinks(exe); e == nil {
			exe = resolved
		}

		return filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "..", ".."))
	}
	wd, _ := os.Getwd()
	return wd
}

func newExecRunner() execRunner { return execRunner{repo: repoRoot()} }

func (e execRunner) Repo() string { return e.repo }

func (e execRunner) Host(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func (e execRunner) Container(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "devcontainer", append([]string{"exec", "--workspace-folder", e.repo}, args...)...)
	cmd.Env = composeEnv(e.repo)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func containerCmd(r Runner, args ...string) *exec.Cmd {
	cmd := exec.Command("devcontainer", append([]string{"exec", "--workspace-folder", r.Repo()}, args...)...)
	cmd.Env = composeEnv(r.Repo())
	return cmd
}
