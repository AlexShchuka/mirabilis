package main

import (
	"context"
	"os"
	"strings"
	"testing"
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

func TestRunProvision_UnknownPhase_Error(t *testing.T) {
	err := runProvision(context.Background(), []string{"--phase", "bogus"})
	if err == nil {
		t.Fatal("runProvision with unknown phase = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown phase") {
		t.Errorf("error = %q, want 'unknown phase'", err.Error())
	}
}

func TestRun_HelpText_HasNotify(t *testing.T) {
	err := run([]string{"--help"})
	if err != nil {
		t.Fatalf("run(--help) = %v, want nil", err)
	}
}

func TestRun_ProvisionSubcmd_UnknownPhase(t *testing.T) {
	err := run([]string{"provision", "--phase", "unknown-phase"})
	if err == nil {
		t.Fatal("run(provision --phase unknown-phase) = nil, want error")
	}
}

func TestRunNotify_UnknownSubcmd(t *testing.T) {
	err := runNotify(context.Background(), []string{"bad"})
	if err == nil {
		t.Fatal("expected error for unknown notify subcommand")
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("error = %q, want 'unknown subcommand'", err.Error())
	}
}

func TestRunNotify_NoArgs(t *testing.T) {
	err := runNotify(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty notify args")
	}
}

func TestResolveRepo_Env(t *testing.T) {
	want := t.TempDir()
	t.Setenv("MIRABILIS_REPO", want)
	got := resolveRepo()
	if got != want {
		t.Errorf("resolveRepo() = %q, want %q", got, want)
	}
}

func TestResolveRepo_Fallback(t *testing.T) {
	t.Setenv("MIRABILIS_REPO", "")
	got := resolveRepo()
	if got == "" {
		t.Error("resolveRepo() returned empty string when MIRABILIS_REPO is unset")
	}
}
