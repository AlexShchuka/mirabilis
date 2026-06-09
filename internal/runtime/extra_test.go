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

func TestHandoffArgv(t *testing.T) {
	argv := handoffArgv("/usr/bin/docker", "ghp_tok", "/tmp/sp.md")
	if len(argv) < 2 {
		t.Fatalf("argv too short: %v", argv)
	}
	if argv[0] != "/usr/bin/docker" {
		t.Errorf("argv[0] = %q, want /usr/bin/docker", argv[0])
	}
	var found bool
	for _, a := range argv {
		if a == "GITHUB_PERSONAL_ACCESS_TOKEN=ghp_tok" {
			found = true
		}
	}
	if !found {
		t.Errorf("token not found in argv: %v", argv)
	}
	last := argv[len(argv)-1]
	if last != "/tmp/sp.md" {
		t.Errorf("last argv = %q, want /tmp/sp.md (system-prompt file)", last)
	}
	found = false
	for _, a := range argv {
		if a == "--append-system-prompt-file" {
			found = true
		}
	}
	if !found {
		t.Errorf("--append-system-prompt-file flag missing in argv: %v", argv)
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

func TestKeychainEnv(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"telegram-token", "TELEGRAM_BOT_TOKEN"},
		{"telegram-chat", "TELEGRAM_CHAT_ID"},
		{"unknown-name", ""},
	}
	for _, tt := range tests {
		got := keychainEnv(tt.name)
		if got != tt.want {
			t.Errorf("keychainEnv(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestKeychainGet_NonDarwinEnvFallback(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok123")
	t.Setenv("TELEGRAM_CHAT_ID", "chat456")

	got := keychainGet("telegram-token")
	if got != "tok123" {
		t.Errorf("keychainGet(telegram-token) = %q, want tok123 (non-darwin env fallback)", got)
	}

	got2 := keychainGet("telegram-chat")
	if got2 != "chat456" {
		t.Errorf("keychainGet(telegram-chat) = %q, want chat456", got2)
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

func TestGitShort(t *testing.T) {
	dir := makeGitRepo(t)
	sha := GitShort(dir)
	if sha == "unknown" {
		t.Fatal("GitShort returned 'unknown' for a valid git repo")
	}
	if len(sha) < 4 {
		t.Errorf("GitShort = %q, expected a short sha (>=4 chars)", sha)
	}

	nonRepo := t.TempDir()
	got := GitShort(nonRepo)
	if got != "unknown" {
		t.Errorf("GitShort(non-repo) = %q, want 'unknown'", got)
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

func dockerInspectEnv(pairs ...string) string {
	var lines []string
	for _, kv := range pairs {
		lines = append(lines, kv)
	}
	return strings.Join(lines, "\n") + "\n"
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

func TestComposeEnv_ManagedKeys(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("STACKS=go,rust\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok-test")
	t.Setenv("TELEGRAM_CHAT_ID", "chat-test")
	t.Setenv("MIRABILIS_VERSION", "old-version")

	env := ComposeEnv(repo)

	counts := map[string]int{}
	values := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		counts[k]++
		values[k] = v
	}

	for _, key := range []string{"MIRABILIS_VERSION", "TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"} {
		if counts[key] > 1 {
			t.Errorf("managed key %s appears %d times, want exactly once", key, counts[key])
		}
	}

	sha := GitShort(repo)
	if values["MIRABILIS_VERSION"] != sha {
		t.Errorf("MIRABILIS_VERSION = %q, want %q", values["MIRABILIS_VERSION"], sha)
	}

	if v := values["STACKS"]; v != "go,rust" {
		t.Errorf("STACKS = %q, want go,rust", v)
	}
}
