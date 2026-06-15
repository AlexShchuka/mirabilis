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
	got := BuildAttachArgv("/tmp/mirabilis-system-prompt.md")
	want := []string{
		"docker", "exec", "-it",
		"-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
		"-e", "COLORTERM=truecolor",
		"-e", "TERM=xterm-256color",
		"mirabilis",
		"claude", "--dangerously-skip-permissions", "--append-system-prompt-file", "/tmp/mirabilis-system-prompt.md",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("attach argv = %v, want %v", got, want)
	}
}

func TestBuildAttachArgvTokenAbsentFromArgv(t *testing.T) {
	t.Parallel()
	argv := BuildAttachArgv("/tmp/spf.md")
	for _, arg := range argv {
		if strings.Contains(arg, "=") && strings.HasPrefix(arg, "GITHUB_PERSONAL_ACCESS_TOKEN=") {
			t.Fatalf("token value leaked into argv element: %q", arg)
		}
		if strings.Contains(arg, "ANTHROPIC") {
			t.Fatalf("argv injects %q", arg)
		}
	}
	found := false
	for i, arg := range argv {
		if arg == "GITHUB_PERSONAL_ACCESS_TOKEN" && i > 0 && argv[i-1] == "-e" {
			found = true
		}
	}
	if !found {
		t.Fatalf("GITHUB_PERSONAL_ACCESS_TOKEN passthrough entry missing from argv: %v", argv)
	}
}

func TestSystemPromptFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := t.TempDir()

	t.Run("returns path on success", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect([]string{"docker", "exec"}, "", nil)
		s := New(fake, NewFakeDocker(), repo)
		got, err := s.SystemPromptFile(ctx)
		if err != nil {
			t.Fatalf("SystemPromptFile error: %v", err)
		}
		if got != "/tmp/mirabilis-system-prompt.md" {
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
		if !strings.Contains(argv[5], "memory_category") {
			t.Fatalf("script missing generated context: %q", argv[5])
		}
	})

	t.Run("surfaces exec error (INV-FAILFAST)", func(t *testing.T) {
		t.Parallel()
		execErr := errors.New("container down")
		fake := exec.NewFake().Expect([]string{"docker", "exec"}, "", execErr)
		s := New(fake, NewFakeDocker(), repo)
		_, err := s.SystemPromptFile(ctx)
		if err == nil {
			t.Fatal("SystemPromptFile returned nil error on exec failure (INV-FAILFAST violated: error must not be swallowed)")
		}
		if !errors.Is(err, execErr) {
			t.Fatalf("SystemPromptFile error = %v, want wrapping %v", err, execErr)
		}
		if len(fake.Calls()) != 1 {
			t.Fatalf("expected exec called once, got %d calls", len(fake.Calls()))
		}
	})
}
