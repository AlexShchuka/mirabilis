package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

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
