package main

import (
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
