package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecRunnerRepo(t *testing.T) {
	want := t.TempDir()
	t.Setenv("MIRABILIS_REPO", want)
	r := NewExecRunner()
	if r.Repo() != want {
		t.Errorf("Repo() = %q, want %q", r.Repo(), want)
	}
}

func TestExecRunnerHost(t *testing.T) {
	want := t.TempDir()
	t.Setenv("MIRABILIS_REPO", want)
	r := NewExecRunner()
	out, err := r.Host(context.Background(), "echo", "hello-host")
	if err != nil {
		t.Fatalf("Host: %v", err)
	}
	if out != "hello-host" {
		t.Errorf("Host echo = %q, want hello-host", out)
	}
}

func TestExecRunnerContainer(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("STACKS=go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	devcontainerDir := makeShim(t, "devcontainer", `printf 'container-ok'`)

	t.Setenv("MIRABILIS_REPO", repo)
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	prependPath(t, devcontainerDir)

	r := NewExecRunner()
	out, err := r.Container(context.Background(), "echo", "hi")
	if err != nil {
		t.Fatalf("Container: %v", err)
	}
	if out != "container-ok" {
		t.Errorf("Container = %q, want container-ok", out)
	}
}

func TestLocalRunnerRepo(t *testing.T) {
	r := NewLocalRunner()
	if r.Repo() != "" {
		t.Errorf("localRunner.Repo() = %q, want empty string", r.Repo())
	}
}

func TestLocalRunnerHost(t *testing.T) {
	r := NewLocalRunner()
	out, err := r.Host(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("localRunner.Host echo: %v", err)
	}
	if out != "hello" {
		t.Errorf("localRunner.Host echo = %q, want hello", out)
	}
}

func TestLocalRunnerContainerNoArgs(t *testing.T) {
	r := NewLocalRunner()
	_, err := r.Container(context.Background())
	if err == nil {
		t.Error("localRunner.Container with no args must return an error")
	}
}

func TestLocalRunnerContainer(t *testing.T) {
	r := NewLocalRunner()
	out, err := r.Container(context.Background(), "echo", "world")
	if err != nil {
		t.Fatalf("localRunner.Container echo: %v", err)
	}
	if out != "world" {
		t.Errorf("localRunner.Container echo = %q, want world", out)
	}
}

func TestRepoRoot_ExecutablePath(t *testing.T) {
	t.Setenv("MIRABILIS_REPO", "")
	got := repoRoot()
	if got == "" {
		t.Error("repoRoot() returned empty string when MIRABILIS_REPO is unset")
	}
}

func TestRepoRoot_EnvOverride(t *testing.T) {
	want := t.TempDir()
	t.Setenv("MIRABILIS_REPO", want)
	got := repoRoot()
	if got != want {
		t.Errorf("repoRoot() = %q, want %q", got, want)
	}
}

func TestWithStderr_WrapsExitErrorStderr(t *testing.T) {
	r := NewLocalRunner()
	_, err := r.Host(context.Background(), "sh", "-c", "echo boom >&2; exit 7")
	if err == nil {
		t.Fatal("Host with failing command returned nil error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q does not contain stderr 'boom'", err.Error())
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Errorf("wrapped error %q no longer satisfies errors.As(*exec.ExitError)", err.Error())
	}
}

func TestWithStderr_PassesThroughNonExitError(t *testing.T) {
	if got := withStderr(nil); got != nil {
		t.Errorf("withStderr(nil) = %v, want nil", got)
	}
	plain := fmt.Errorf("plain error")
	if got := withStderr(plain); got != plain {
		t.Errorf("withStderr(plain) = %v, want the same error unchanged", got)
	}
}
