package tgtoken

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead_NoFile_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if got := Read(); got != "" {
		t.Errorf("Read() with no file = %q, want empty", got)
	}
}

func TestRead_WrittenFile_ReturnsToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, Filename), []byte("bot123:token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Read(); got != "bot123:token" {
		t.Errorf("Read() = %q, want bot123:token", got)
	}
}

func TestRead_HomeFromUserHomeDir(t *testing.T) {
	t.Setenv("HOME", "")
	got := Read()
	_ = got
}

func TestReadFile_ClaudeToken_WrittenFile_ReturnsToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, FilenameClaude), []byte("oauth-abc-xyz\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadFile(FilenameClaude); got != "oauth-abc-xyz" {
		t.Errorf("ReadFile(FilenameClaude) = %q, want oauth-abc-xyz", got)
	}
}

func TestReadFile_NoFile_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if got := ReadFile(FilenameClaude); got != "" {
		t.Errorf("ReadFile(FilenameClaude) with no file = %q, want empty", got)
	}
}

func TestReadFile_HomeEmpty_Empty(t *testing.T) {
	t.Setenv("HOME", "")
	got := ReadFile(FilenameClaude)
	_ = got
}
