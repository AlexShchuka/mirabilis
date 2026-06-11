package provision

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestWriteClaudeToken_CreatesFileWith0600(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := WriteClaudeToken("oauth-test-token"); err != nil {
		t.Fatalf("WriteClaudeToken = %v, want nil", err)
	}

	path := ClaudeTokenPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("token file not created: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestWriteClaudeToken_ContentRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	token := "oauth-round-trip"
	if err := WriteClaudeToken(token); err != nil {
		t.Fatalf("WriteClaudeToken = %v, want nil", err)
	}

	got := ReadClaudeTokenFile()
	if got != token {
		t.Errorf("ReadClaudeTokenFile() = %q, want %q", got, token)
	}
}

func TestWriteClaudeToken_Overwrite(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := WriteClaudeToken("first-token"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteClaudeToken("second-token"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got := ReadClaudeTokenFile()
	if got != "second-token" {
		t.Errorf("ReadClaudeTokenFile() after overwrite = %q, want second-token", got)
	}
}

func TestReadClaudeTokenFile_AbsentReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := ReadClaudeTokenFile()
	if got != "" {
		t.Errorf("ReadClaudeTokenFile() with no file = %q, want empty", got)
	}
}

func TestClaudeTokenPath_InsideDotClaude(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path := ClaudeTokenPath()
	want := filepath.Join(tmp, ".claude", FileClaudeToken)
	if path != want {
		t.Errorf("ClaudeTokenPath() = %q, want %q", path, want)
	}
	if !strings.HasPrefix(path, tmp) {
		t.Errorf("ClaudeTokenPath() = %q, want under HOME %q", path, tmp)
	}
}

func TestWriteClaudeToken_RenameToDirectory_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(cd, FileClaudeToken)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteClaudeToken("tok"); err == nil {
		t.Error("WriteClaudeToken when destination is a directory = nil, want error")
	}
	_ = os.Remove(dest)
}

func TestWriteClaudeToken_MkdirFails_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	tmp := t.TempDir()
	ro := filepath.Join(tmp, "ro")
	if err := os.MkdirAll(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	t.Setenv("HOME", filepath.Join(ro, "home"))

	if err := WriteClaudeToken("some-token"); err == nil {
		t.Error("WriteClaudeToken in read-only home = nil, want error")
	}
}

func TestWriteClaudeToken_CreateTempFails_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := WriteClaudeToken("first-token"); err != nil {
		t.Fatalf("setup write: %v", err)
	}
	cd := filepath.Join(tmp, ".claude")
	if err := os.Chmod(cd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cd, 0o755) })

	if err := WriteClaudeToken("second-token"); err == nil {
		t.Error("WriteClaudeToken with read-only claude dir = nil, want error")
	}
}

func TestClaudeCredentialsConflict_NotRunning_ReturnsFalse(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(_ []string) (string, error) {
			return "", nil
		},
	}
	if got := ClaudeCredentialsConflict(context.Background(), r); got {
		t.Error("ClaudeCredentialsConflict with container returning empty = true, want false")
	}
}

func TestClaudeCredentialsConflict_NoCredentials_ReturnsFalse(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(_ []string) (string, error) {
			return "no\n", nil
		},
	}
	if got := ClaudeCredentialsConflict(context.Background(), r); got {
		t.Error("ClaudeCredentialsConflict with 'no' output = true, want false")
	}
}

func TestClaudeCredentialsConflict_CredentialsPresent_ReturnsTrue(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(_ []string) (string, error) {
			return "yes\n", nil
		},
	}
	if got := ClaudeCredentialsConflict(context.Background(), r); !got {
		t.Error("ClaudeCredentialsConflict with 'yes' output = false, want true")
	}
}
