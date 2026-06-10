package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func makeShim(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func prependPath(t *testing.T, dirs ...string) {
	t.Helper()
	base := os.Getenv("PATH")
	prefix := ""
	for _, d := range dirs {
		if prefix != "" {
			prefix += ":"
		}
		prefix += d
	}
	t.Setenv("PATH", prefix+":"+base)
}

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

func TestContainerCmd(t *testing.T) {
	repo := t.TempDir()
	r := &runner.FakeRunner{RepoVal: repo}
	ctx := context.Background()
	cmd := ContainerCmd(ctx, r, "bash", "-c", "echo hi")
	if cmd == nil {
		t.Fatal("ContainerCmd returned nil")
	}
	if len(cmd.Args) < 2 {
		t.Fatalf("ContainerCmd args too short: %v", cmd.Args)
	}
}

func TestDockerReachable_False(t *testing.T) {
	shimDir := makeShim(t, "docker", `exit 1`)
	prependPath(t, shimDir)
	if dockerReachable() {
		t.Error("dockerReachable should return false when docker exits non-zero")
	}
}

func TestDockerReachable_True(t *testing.T) {
	shimDir := makeShim(t, "docker", `exit 0`)
	prependPath(t, shimDir)
	if !dockerReachable() {
		t.Error("dockerReachable should return true when docker exits zero")
	}
}

func TestEnsureDocker_DockerMissing(t *testing.T) {
	shimDir := t.TempDir()
	t.Setenv("PATH", shimDir)
	err := EnsureDocker(context.Background())
	if err == nil {
		t.Error("EnsureDocker must error when docker is missing from PATH")
	}
}

func TestEnsureDocker_DevcontainerMissing(t *testing.T) {
	dockerDir := makeShim(t, "docker", `exit 0`)
	emptyDir := t.TempDir()
	t.Setenv("PATH", dockerDir+":"+emptyDir)
	err := EnsureDocker(context.Background())
	if err == nil {
		t.Error("EnsureDocker must error when devcontainer is missing")
	}
}

func TestEnsureDocker_BothPresentReachable(t *testing.T) {
	dir1 := makeShim(t, "docker", `exit 0`)
	dir2 := makeShim(t, "devcontainer", `exit 0`)
	prependPath(t, dir1, dir2)
	err := EnsureDocker(context.Background())
	if err != nil {
		t.Errorf("EnsureDocker = %v, want nil when docker reachable", err)
	}
}

func TestResetAll_Success(t *testing.T) {
	repo := t.TempDir()
	dockerDir := makeShim(t, "docker", `exit 0`)
	prependPath(t, dockerDir)
	t.Setenv("MIRABILIS_REPO", repo)

	r := &runner.FakeRunner{RepoVal: repo}
	err := ResetAll(context.Background(), r)
	if err != nil {
		t.Errorf("ResetAll = %v, want nil on docker success", err)
	}
}

func TestResetAll_Failure(t *testing.T) {
	repo := t.TempDir()
	dockerDir := makeShim(t, "docker", `echo "compose down error"; exit 1`)
	prependPath(t, dockerDir)
	t.Setenv("MIRABILIS_REPO", repo)

	r := &runner.FakeRunner{RepoVal: repo}
	err := ResetAll(context.Background(), r)
	if err == nil {
		t.Error("ResetAll must return error when docker compose down fails")
	}
}

func TestResolveCode_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := resolveCode()
	if err == nil {
		t.Error("resolveCode must error when code binary not on PATH and no app bundle found")
	}
}

func TestResolveCode_OnPath(t *testing.T) {
	codeDir := makeShim(t, "code", `exit 0`)
	prependPath(t, codeDir)
	got, err := resolveCode()
	if err != nil {
		t.Fatalf("resolveCode: %v", err)
	}
	want := filepath.Join(codeDir, "code")
	if got != want {
		t.Errorf("resolveCode = %q, want %q", got, want)
	}
}

func TestResolveCode_FlatpakHomeBundle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", t.TempDir())
	bundle := filepath.Join(tmp, ".local/share/flatpak/exports/bin/com.visualstudio.code")
	if err := os.MkdirAll(filepath.Dir(bundle), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundle, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveCode()
	if err != nil {
		t.Fatalf("resolveCode: %v", err)
	}
	if got != bundle {
		t.Errorf("resolveCode = %q, want %q", got, bundle)
	}
}

func TestDoVSCode_CodeNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
	}
	err := DoVSCode(context.Background(), r)
	if err == nil {
		t.Error("DoVSCode must error when code binary is not found")
	}
}

func TestDoVSCode_ContainerNotRunning(t *testing.T) {
	codeDir := makeShim(t, "code", `exit 0`)
	devcontainerDir := makeShim(t, "devcontainer", `exit 0`)
	prependPath(t, codeDir, devcontainerDir)

	repo := t.TempDir()
	r := &runner.FakeRunner{
		RepoVal: repo,
		HostFunc: func(name string, args []string) (string, error) {
			return "false", nil
		},
	}
	err := DoVSCode(context.Background(), r)
	if err != nil {
		t.Errorf("DoVSCode (container not running, devcontainer succeeds) = %v, want nil", err)
	}
}

func TestDoVSCode_ContainerRunningLaunchesCode(t *testing.T) {
	codeDir := makeShim(t, "code", `exit 0`)
	prependPath(t, codeDir)

	repo := t.TempDir()
	r := &runner.FakeRunner{
		RepoVal: repo,
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
	}
	err := DoVSCode(context.Background(), r)
	if err != nil {
		t.Errorf("DoVSCode (container running) = %v, want nil", err)
	}
}

func TestDoVSCode_DevcontainerUpFails(t *testing.T) {
	codeDir := makeShim(t, "code", `exit 0`)
	devcontainerDir := makeShim(t, "devcontainer", `exit 1`)
	prependPath(t, codeDir, devcontainerDir)

	repo := t.TempDir()
	r := &runner.FakeRunner{
		RepoVal: repo,
		HostFunc: func(name string, args []string) (string, error) {
			return "false", nil
		},
	}
	err := DoVSCode(context.Background(), r)
	if err == nil {
		t.Error("DoVSCode must return error when devcontainer up fails")
	}
}

func TestKeychainGet_UnknownName(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	got := keychainGet("unknown-name")
	if got != "" {
		t.Errorf("keychainGet(unknown-name) = %q, want empty", got)
	}
}

func TestKeychainGet_AccountOverride(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "myaccount")
	t.Setenv("TELEGRAM_BOT_TOKEN", "overridden")
	got := keychainGet("telegram-token")
	if got != "overridden" {
		t.Errorf("keychainGet with account override = %q, want overridden", got)
	}
}

func TestRepoRoot_ExecutablePath(t *testing.T) {
	t.Setenv("MIRABILIS_REPO", "")
	got := repoRoot()
	if got == "" {
		t.Error("repoRoot() returned empty string when MIRABILIS_REPO is unset")
	}
}

func TestComposeEnv_EmptyManagedValuesOmitted(t *testing.T) {
	repo := makeGitRepo(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	env := ComposeEnv(repo)
	for _, kv := range env {
		if kv == "TELEGRAM_BOT_TOKEN=" || kv == "TELEGRAM_CHAT_ID=" {
			t.Errorf("empty managed key must not appear in env: %q", kv)
		}
	}
}
