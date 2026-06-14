package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const vscodeURI = "vscode-remote://attached-container+" +
	"7b22636f6e7461696e65724e616d65223a222f6d69726162696c6973227d/"

func fakeCodeBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "code")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return path
}

func TestVSCodeArgv(t *testing.T) {
	t.Parallel()
	got := vscodeArgv("/usr/local/bin/code")
	want := []string{"/usr/local/bin/code", "--folder-uri", vscodeURI}
	if !slices.Equal(got, want) {
		t.Fatalf("vscode argv = %v, want %v", got, want)
	}
}

func TestOpenVSCodeRunning(t *testing.T) {
	code := fakeCodeBinary(t)
	repo := t.TempDir()
	fd := NewFakeDocker().StubInspect(Container{Running: true}, nil)
	fake := exec.NewFake().Expect([]string{code}, "", nil)
	s := New(fake, fd, repo)
	if err := s.OpenVSCode(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1: %v", len(calls), calls)
	}
	if !slices.Equal(calls[0].Argv, []string{code, "--folder-uri", vscodeURI}) {
		t.Fatalf("argv = %v", calls[0].Argv)
	}
}

func TestOpenVSCodeStartsContainer(t *testing.T) {
	code := fakeCodeBinary(t)
	repo := t.TempDir()
	fd := NewFakeDocker().StubInspect(Container{Running: false}, nil)
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc1234", nil).
		Expect([]string{"docker", "compose"}, "", nil).
		Expect([]string{code}, "", nil)
	s := New(fake, fd, repo)
	if err := s.OpenVSCode(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3: %v", len(calls), calls)
	}
	wantUp := []string{"docker", "compose", "-f", "docker-compose.yml", "up", "-d"}
	if !slices.Equal(calls[1].Argv, wantUp) {
		t.Fatalf("up argv = %v, want %v", calls[1].Argv, wantUp)
	}
	if !slices.Equal(calls[2].Argv, []string{code, "--folder-uri", vscodeURI}) {
		t.Fatalf("code argv = %v", calls[2].Argv)
	}
}
