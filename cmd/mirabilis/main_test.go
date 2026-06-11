package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestEffectiveVersion_ldflags(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })
	version = "abc1234"
	if got := effectiveVersion(); got != "abc1234" {
		t.Fatalf("expected abc1234, got %q", got)
	}
}

func TestEffectiveVersion_env(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })
	version = "unknown"
	t.Setenv("MIRABILIS_VERSION", "env-sha")
	if got := effectiveVersion(); got != "env-sha" {
		t.Fatalf("expected env-sha, got %q", got)
	}
}

func TestEffectiveVersion_fallback(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })
	version = "unknown"
	os.Unsetenv("MIRABILIS_VERSION")
	if got := effectiveVersion(); got != "unknown" {
		t.Fatalf("expected unknown, got %q", got)
	}
}

func TestRun_versionFlags(t *testing.T) {
	for _, arg := range []string{"-version", "--version", "version"} {
		t.Run(arg, func(t *testing.T) {
			if err := run([]string{arg}); err != nil {
				t.Fatalf("run(%q) returned unexpected error: %v", arg, err)
			}
		})
	}
}

func TestRun_helpFlags(t *testing.T) {
	for _, arg := range []string{"-h", "-help", "--help", "help"} {
		t.Run(arg, func(t *testing.T) {
			if err := run([]string{arg}); err != nil {
				t.Fatalf("run(%q) returned unexpected error: %v", arg, err)
			}
		})
	}
}

func TestRun_unknownArg(t *testing.T) {
	err := run([]string{"bogus-arg"})
	if err == nil {
		t.Fatal("expected error for unknown arg, got nil")
	}
	if !strings.Contains(err.Error(), "unknown argument") {
		t.Fatalf("expected 'unknown argument' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--help") {
		t.Fatalf("expected '--help' suggestion in error, got: %v", err)
	}
}

func TestRun_hookMissingName(t *testing.T) {
	err := run([]string{"hook"})
	if err == nil {
		t.Fatal("expected error for hook without name, got nil")
	}
	if !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("expected 'missing name' in error, got: %v", err)
	}
}

func TestRun_hookUnknownName(t *testing.T) {
	err := run([]string{"hook", "no-such-hook-xyz"})
	if err == nil {
		t.Fatal("expected error for unknown hook name, got nil")
	}
	if !strings.Contains(err.Error(), "unknown hook") {
		t.Errorf("error = %q, want 'unknown hook'", err.Error())
	}
}

func TestRunTgOutbox_MissingTokenFile_Error(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	err := runTgOutbox(context.Background(), nil)
	if err == nil {
		t.Fatal("runTgOutbox with missing token = nil, want error")
	}
	if !strings.Contains(err.Error(), "bot token not found") {
		t.Errorf("error = %q, want 'bot token not found'", err.Error())
	}
}

func TestRunTgOutbox_EmptyChatID_Error(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	claudeDir := tmp + "/.claude"
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tokenPath := claudeDir + "/.mirabilis-telegram-token"
	if err := os.WriteFile(tokenPath, []byte("fake-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELEGRAM_CHAT_ID", "")

	err := runTgOutbox(context.Background(), nil)
	if err == nil {
		t.Fatal("runTgOutbox with empty chat ID = nil, want error")
	}
	if !strings.Contains(err.Error(), "telegram-chat not configured") {
		t.Errorf("error = %q, want 'telegram-chat not configured'", err.Error())
	}
}

func TestRunProvision_UnknownPhase_Error(t *testing.T) {
	err := runProvision(context.Background(), &runner.FakeRunner{}, []string{"--phase", "bogus"})
	if err == nil {
		t.Fatal("runProvision with unknown phase = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown --phase") {
		t.Errorf("error = %q, want 'unknown --phase'", err.Error())
	}
}

func TestRun_HelpText_IncludesTgOutbox(t *testing.T) {
	err := run([]string{"--help"})
	if err != nil {
		t.Fatalf("run(--help) = %v, want nil", err)
	}
}

func TestRun_TgOutbox_MissingToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	err := run([]string{"tg-outbox"})
	if err == nil {
		t.Fatal("run(tg-outbox) with no token = nil, want error")
	}
}

func TestRunTgOutbox_WithTokenAndChat_StartsWatcher(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	claudeDir := tmp + "/.claude"
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tokenPath := claudeDir + "/.mirabilis-telegram-token"
	if err := os.WriteFile(tokenPath, []byte("fake-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELEGRAM_CHAT_ID", "-100watcher")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runTgOutbox(ctx, nil)
	if err != nil {
		t.Errorf("runTgOutbox with valid token+chat = %v, want nil", err)
	}
}

func TestRunProvision_PluginsPhase_FakeRunner(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		HostFunc: func(_ string, _ []string) (string, error) {
			return "", nil
		},
		ContFunc: func(_ []string) (string, error) {
			return "", nil
		},
	}
	if err := runProvision(context.Background(), r, []string{"--phase", "plugins"}); err != nil {
		t.Errorf("runProvision plugins with FakeRunner = %v, want nil", err)
	}
}

func TestRunProvision_ProvisionSubcmd_UnknownPhase(t *testing.T) {
	err := run([]string{"provision", "--phase", "unknown-phase"})
	if err == nil {
		t.Fatal("run(provision --phase unknown-phase) = nil, want error")
	}
}

func TestRunProvision_SkillsPhase_EmptyConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := runProvision(context.Background(), &runner.FakeRunner{}, []string{"--phase", "skills"}); err != nil {
		t.Errorf("runProvision skills with empty config = %v, want nil", err)
	}
}

func TestRunProvision_CreatePhase_FakeRunner(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		HostFunc: func(_ string, _ []string) (string, error) {
			return "", nil
		},
		ContFunc: func(_ []string) (string, error) {
			return "", nil
		},
	}
	if err := runProvision(context.Background(), r, []string{"--phase", "create"}); err != nil {
		t.Errorf("runProvision create with FakeRunner = %v, want nil", err)
	}
}

func TestRunProvision_StartPhase_FakeRunner(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		HostFunc: func(_ string, _ []string) (string, error) {
			return "", nil
		},
		ContFunc: func(_ []string) (string, error) {
			return "", nil
		},
	}
	if err := runProvision(context.Background(), r, []string{"--phase", "start"}); err != nil {
		t.Errorf("runProvision start with FakeRunner = %v, want nil", err)
	}
}
