package sandbox

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func TestBuildAttachArgv(t *testing.T) {
	t.Parallel()
	got := BuildAttachArgv("/tmp/mirabilis-system-prompt.md", "ghp_token123")
	want := []string{
		"docker", "exec", "-it",
		"-e", "GITHUB_PERSONAL_ACCESS_TOKEN=ghp_token123",
		"-e", "COLORTERM=truecolor",
		"-e", "TERM=xterm-256color",
		"mirabilis",
		"claude", "--dangerously-skip-permissions", "--append-system-prompt-file", "/tmp/mirabilis-system-prompt.md",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("attach argv = %v, want %v", got, want)
	}
}

func TestBuildAttachArgvTokenPlacement(t *testing.T) {
	t.Parallel()
	argv := BuildAttachArgv("/tmp/spf.md", "ghp_token123")
	hits := 0
	for i, arg := range argv {
		if !strings.Contains(arg, "ghp_token123") {
			continue
		}
		hits++
		if arg != "GITHUB_PERSONAL_ACCESS_TOKEN=ghp_token123" {
			t.Fatalf("token in unexpected form: %q", arg)
		}
		if i == 0 || argv[i-1] != "-e" {
			t.Fatalf("token entry not preceded by -e: %v", argv)
		}
	}
	if hits != 1 {
		t.Fatalf("token appears %d times in argv, want 1", hits)
	}
	for _, arg := range argv {
		if strings.Contains(arg, "ANTHROPIC") {
			t.Fatalf("argv injects %q", arg)
		}
	}
}

func TestSystemPromptFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := t.TempDir()

	t.Run("resolved", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect([]string{"docker", "exec"}, "/tmp/mirabilis-system-prompt.md", nil)
		s := New(fake, NewFakeDocker(), repo)
		if got := s.SystemPromptFile(ctx); got != "/tmp/mirabilis-system-prompt.md" {
			t.Fatalf("SystemPromptFile = %q", got)
		}
		calls := fake.Calls()
		if len(calls) != 1 {
			t.Fatalf("got %d calls, want 1", len(calls))
		}
		argv := calls[0].Argv
		wantPrefix := []string{"docker", "exec", "mirabilis", "bash", "-lc"}
		if len(argv) != 6 || !slices.Equal(argv[:5], wantPrefix) {
			t.Fatalf("argv = %v", argv)
		}
		if !strings.Contains(argv[5], "sandbox-context.md") {
			t.Fatalf("script missing: %q", argv[5])
		}
	})

	t.Run("fallback on error", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect([]string{"docker", "exec"}, "", errors.New("container down"))
		s := New(fake, NewFakeDocker(), repo)
		if got := s.SystemPromptFile(ctx); got != "/opt/mirabilis/config/sandbox-context.md" {
			t.Fatalf("SystemPromptFile = %q", got)
		}
	})

	t.Run("fallback on empty", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect([]string{"docker", "exec"}, "", nil)
		s := New(fake, NewFakeDocker(), repo)
		if got := s.SystemPromptFile(ctx); got != "/opt/mirabilis/config/sandbox-context.md" {
			t.Fatalf("SystemPromptFile = %q", got)
		}
	})
}
