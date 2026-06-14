//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func makeBin(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return p
}

func TestCopyTextXclipUsesClipboardSelection(t *testing.T) {
	xclip := makeBin(t, "xclip")
	f := exec.NewFake()
	f.Expect([]string{xclip, "-selection", "clipboard"}, "", nil)

	if err := CopyText(t.Context(), f, "hello"); err != nil {
		t.Fatalf("CopyText: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("unexpected remaining stubs: %d", n)
	}
}

func TestCopyTextXselUsesClipboardInput(t *testing.T) {
	xsel := makeBin(t, "xsel")
	f := exec.NewFake()
	f.Expect([]string{xsel, "--clipboard", "--input"}, "", nil)

	if err := CopyText(t.Context(), f, "hello"); err != nil {
		t.Fatalf("CopyText: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("unexpected remaining stubs: %d", n)
	}
}

func TestCopyTextNoClipboardReturnsError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	f := exec.NewFake()
	err := CopyText(t.Context(), f, "hello")
	if err == nil {
		t.Fatal("CopyText with no clipboard tools = nil, want ErrNoClipboard")
	}
}
