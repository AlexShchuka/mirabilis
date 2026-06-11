package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, parts := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
	} {
		if err := runGitCmd(dir, parts[1:]...); err != nil {
			t.Fatalf("git %v: %v", parts[1:], err)
		}
	}
	empty := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, parts := range [][]string{
		{"add", ".gitkeep"},
		{"commit", "-m", "init"},
	} {
		if err := runGitCmd(dir, parts...); err != nil {
			t.Fatalf("git %v: %v", parts, err)
		}
	}
	return dir
}

func runGitCmd(dir string, args ...string) error {
	out, err := os.StartProcess("/usr/bin/git", append([]string{"git"}, args...), &os.ProcAttr{
		Dir:   dir,
		Files: []*os.File{nil, nil, nil},
	})
	if err != nil {
		return err
	}
	state, err := out.Wait()
	if err != nil {
		return err
	}
	if !state.Success() {
		return fmt.Errorf("git %v: exit %d", args, state.ExitCode())
	}
	return nil
}

func dockerInspectEnv(pairs ...string) string {
	var lines []string
	lines = append(lines, pairs...)
	return strings.Join(lines, "\n") + "\n"
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
	err := ResetAll(context.Background(), r, false)
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
	err := ResetAll(context.Background(), r, false)
	if err == nil {
		t.Error("ResetAll must return error when docker compose down fails")
	}
}

func TestResetAll_Preserve_SavesMemoryBeforeDown(t *testing.T) {
	repo := t.TempDir()
	dockerDir := makeShim(t, "docker", `exit 0`)
	prependPath(t, dockerDir)
	t.Setenv("MIRABILIS_REPO", repo)

	r := &runner.FakeRunner{RepoVal: repo}
	err := ResetAll(context.Background(), r, true)
	if err != nil {
		t.Errorf("ResetAll preserve = %v, want nil", err)
	}
}

func TestRestoreMemoryFromHost_NoSnapshot_Noop(t *testing.T) {
	repo := t.TempDir()
	r := &runner.FakeRunner{
		RepoVal: repo,
		HostFunc: func(name string, args []string) (string, error) {
			t.Errorf("unexpected Host call: %s %v", name, args)
			return "", nil
		},
	}
	if err := RestoreMemoryFromHost(context.Background(), r); err != nil {
		t.Errorf("RestoreMemoryFromHost with no snapshot = %v, want nil", err)
	}
}

func TestRestoreMemoryFromHost_DockerCp_InvocationAndCleanup(t *testing.T) {
	repo := t.TempDir()
	savePath := MemorySavePath(repo)
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savePath, "sandbox-ops.md"), []byte("---\ncategory: sandbox-ops\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var capturedName string
	var capturedArgs []string
	r := &runner.FakeRunner{
		RepoVal: repo,
		HostFunc: func(name string, args []string) (string, error) {
			capturedName = name
			capturedArgs = args
			return "", nil
		},
	}

	if err := RestoreMemoryFromHost(context.Background(), r); err != nil {
		t.Fatalf("RestoreMemoryFromHost = %v, want nil", err)
	}

	if capturedName != "docker" {
		t.Errorf("Host called with %q, want %q", capturedName, "docker")
	}
	wantArgs := []string{"cp", savePath + "/.", "mirabilis:/home/node/.claude/memory/"}
	if len(capturedArgs) != len(wantArgs) {
		t.Fatalf("docker cp args = %v, want %v", capturedArgs, wantArgs)
	}
	for i, want := range wantArgs {
		if capturedArgs[i] != want {
			t.Errorf("arg[%d] = %q, want %q", i, capturedArgs[i], want)
		}
	}

	if _, err := os.Stat(savePath); err == nil {
		t.Error("staging dir must be removed after docker cp")
	}
}

func TestRestoreMemoryFromHost_DockerCpError_Propagated(t *testing.T) {
	repo := t.TempDir()
	savePath := MemorySavePath(repo)
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &runner.FakeRunner{
		RepoVal: repo,
		HostFunc: func(name string, args []string) (string, error) {
			return "", fmt.Errorf("container not running")
		},
	}

	err := RestoreMemoryFromHost(context.Background(), r)
	if err == nil {
		t.Error("RestoreMemoryFromHost must return error when docker cp fails")
	}
	if !strings.Contains(err.Error(), "restore memory") {
		t.Errorf("error %v does not mention 'restore memory'", err)
	}
}

func TestContainerRunning(t *testing.T) {
	tests := []struct {
		name    string
		hostOut string
		want    bool
	}{
		{"true output", "true", true},
		{"false output", "false", false},
		{"empty output", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runner.FakeRunner{
				HostFunc: func(name string, args []string) (string, error) {
					return tt.hostOut, nil
				},
			}
			got := ContainerRunning(context.Background(), r)
			if got != tt.want {
				t.Errorf("ContainerRunning = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerExists(t *testing.T) {
	tests := []struct {
		name    string
		hostErr error
		want    bool
	}{
		{"no error means exists", nil, true},
		{"error means not exists", fmt.Errorf("not found"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runner.FakeRunner{
				HostFunc: func(name string, args []string) (string, error) {
					return "", tt.hostErr
				},
			}
			got := ContainerExists(context.Background(), r)
			if got != tt.want {
				t.Errorf("ContainerExists = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerEnvValue(t *testing.T) {
	tests := []struct {
		name    string
		hostOut string
		hostErr error
		key     string
		want    string
	}{
		{
			name:    "key found",
			hostOut: "FOO=bar\nBAZ=qux\n",
			key:     "FOO",
			want:    "bar",
		},
		{
			name:    "key not found",
			hostOut: "FOO=bar\n",
			key:     "BAZ",
			want:    "",
		},
		{
			name:    "host error returns empty",
			hostErr: fmt.Errorf("docker error"),
			key:     "FOO",
			want:    "",
		},
		{
			name:    "prefix collision: FOO vs FOOBAR",
			hostOut: "FOOBAR=wrong\nFOO=correct\n",
			key:     "FOO",
			want:    "correct",
		},
		{
			name:    "key with empty value",
			hostOut: "FOO=\n",
			key:     "FOO",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runner.FakeRunner{
				HostFunc: func(name string, args []string) (string, error) {
					return tt.hostOut, tt.hostErr
				},
			}
			got := ContainerEnvValue(context.Background(), r, tt.key)
			if got != tt.want {
				t.Errorf("ContainerEnvValue(_, _, %q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestLastLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{
			name:  "fewer lines than n",
			input: "a\nb",
			n:     5,
			want:  "a\nb",
		},
		{
			name:  "exactly n lines",
			input: "a\nb\nc",
			n:     3,
			want:  "a\nb\nc",
		},
		{
			name:  "more than n",
			input: "a\nb\nc\nd\ne",
			n:     3,
			want:  "c\nd\ne",
		},
		{
			name:  "trailing newline trimmed",
			input: "a\nb\n",
			n:     5,
			want:  "a\nb",
		},
		{
			name:  "single line",
			input: "hello",
			n:     1,
			want:  "hello",
		},
		{
			name:  "empty string",
			input: "",
			n:     3,
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LastLines(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("LastLines(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	ctx := context.Background()

	t.Run("container version empty → true", func(t *testing.T) {
		r := &runner.FakeRunner{
			HostFunc: func(name string, args []string) (string, error) {
				return dockerInspectEnv("OTHER=val"), nil
			},
		}
		if !IsStale(ctx, r) {
			t.Error("IsStale must be true when MIRABILIS_VERSION is absent")
		}
	})

	t.Run("git rev-parse error → false", func(t *testing.T) {
		repo := t.TempDir()
		r := &runner.FakeRunner{
			RepoVal: repo,
			HostFunc: func(name string, args []string) (string, error) {
				if name == "docker" {
					return dockerInspectEnv("MIRABILIS_VERSION=abc123", "MIRABILIS_STACKS="), nil
				}
				return "", fmt.Errorf("not a git repo")
			},
		}
		got := IsStale(ctx, r)
		if got {
			t.Error("IsStale must be false when git rev-parse errors")
		}
	})

	t.Run("same sha → false", func(t *testing.T) {
		repo := makeGitRepo(t)
		sha := GitShort(repo)
		r := &runner.FakeRunner{
			RepoVal: repo,
			HostFunc: func(name string, args []string) (string, error) {
				if name == "docker" {
					return dockerInspectEnv("MIRABILIS_VERSION="+sha, "MIRABILIS_STACKS="), nil
				}
				return sha, nil
			},
		}
		if IsStale(ctx, r) {
			t.Error("IsStale = true when container sha equals repo sha")
		}
	})

	t.Run("differing sha → true", func(t *testing.T) {
		repo := makeGitRepo(t)
		r := &runner.FakeRunner{
			RepoVal: repo,
			HostFunc: func(name string, args []string) (string, error) {
				if name == "docker" {
					return dockerInspectEnv("MIRABILIS_VERSION=aaa111", "MIRABILIS_STACKS="), nil
				}
				return "bbb222", nil
			},
		}
		if !IsStale(ctx, r) {
			t.Error("IsStale = false when container sha differs from repo sha")
		}
	})

	t.Run("STACKS mismatch → true", func(t *testing.T) {
		repo := makeGitRepo(t)
		sha := GitShort(repo)
		if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("STACKS=go\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := &runner.FakeRunner{
			RepoVal: repo,
			HostFunc: func(name string, args []string) (string, error) {
				if name == "docker" {
					return dockerInspectEnv("MIRABILIS_VERSION="+sha, "MIRABILIS_STACKS=rust"), nil
				}
				return sha, nil
			},
		}
		if !IsStale(ctx, r) {
			t.Error("IsStale = false when STACKS value differs")
		}
	})
}
