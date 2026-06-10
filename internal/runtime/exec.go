package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

var _ runner.Runner = (*execRunner)(nil)
var _ runner.Runner = (*localRunner)(nil)

type execRunner struct{ repo string }

func NewExecRunner() runner.Runner { return &execRunner{repo: repoRoot()} }

func (e *execRunner) Repo() string { return e.repo }

func (e *execRunner) Host(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), withStderr(err)
}

func (e *execRunner) Container(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "devcontainer", append([]string{"exec", "--workspace-folder", e.repo}, args...)...)
	cmd.Env = ComposeEnv(e.repo)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), withStderr(err)
}

type localRunner struct{}

func NewLocalRunner() runner.Runner { return &localRunner{} }

func (l *localRunner) Repo() string { return "" }

func (l *localRunner) Host(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), withStderr(err)
}

func (l *localRunner) Container(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("localRunner.Container: no args")
	}
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	return strings.TrimSpace(string(out)), withStderr(err)
}

func withStderr(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(bytes.TrimSpace(ee.Stderr)) > 0 {
		return fmt.Errorf("%w: %s", err, bytes.TrimSpace(ee.Stderr))
	}
	return err
}

func repoRoot() string {
	if r := os.Getenv("MIRABILIS_REPO"); r != "" {
		return r
	}
	exe, err := os.Executable()
	if err == nil {
		if resolved, e := filepath.EvalSymlinks(exe); e == nil {
			exe = resolved
		}
		return filepath.Clean(filepath.Join(filepath.Dir(exe), ".."))
	}
	wd, _ := os.Getwd()
	return wd
}
