package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTelegramToken_CreatesFileWith0600(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := WriteTelegramToken("bot123:abc"); err != nil {
		t.Fatalf("WriteTelegramToken = %v, want nil", err)
	}

	path := TelegramTokenPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("token file not created: %v", err)
	}
	// Verify 0600 permissions.
	if info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestWriteTelegramToken_ContentRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	token := "bot999:XYZ"
	if err := WriteTelegramToken(token); err != nil {
		t.Fatalf("WriteTelegramToken = %v, want nil", err)
	}

	got := ReadTelegramTokenFile()
	if got != token {
		t.Errorf("ReadTelegramTokenFile() = %q, want %q", got, token)
	}
}

func TestWriteTelegramToken_Overwrite(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := WriteTelegramToken("first:token"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteTelegramToken("second:token"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got := ReadTelegramTokenFile()
	if got != "second:token" {
		t.Errorf("ReadTelegramTokenFile() after overwrite = %q, want second:token", got)
	}
}

func TestReadTelegramTokenFile_AbsentReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := ReadTelegramTokenFile()
	if got != "" {
		t.Errorf("ReadTelegramTokenFile() with no file = %q, want empty", got)
	}
}

func TestTelegramTokenPath_NotInRepo(t *testing.T) {
	// The token path must be outside any repo working tree.
	// It must be inside ~/.claude — which is excluded from git via .gitignore.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path := TelegramTokenPath()

	// Must be under HOME (not a global system path, not a workspace path).
	if !strings.HasPrefix(path, tmp) {
		t.Errorf("TelegramTokenPath() = %q, want to be under HOME %q", path, tmp)
	}

	// Must be inside .claude subdirectory.
	want := filepath.Join(tmp, ".claude", FileTelegramToken)
	if path != want {
		t.Errorf("TelegramTokenPath() = %q, want %q", path, want)
	}
}

func TestWriteTelegramToken_RenameToDirectory_ReturnsError(t *testing.T) {
	// Make the destination path be a directory to force Rename to fail.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a directory where the token file would land so Rename fails.
	dest := filepath.Join(cd, FileTelegramToken)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteTelegramToken("bot:test"); err == nil {
		t.Error("WriteTelegramToken when destination is a directory = nil, want error")
	}
	// Cleanup: remove the directory we created.
	_ = os.Remove(dest)
}

func TestWriteTelegramToken_MkdirFails_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	tmp := t.TempDir()
	// Make HOME a read-only directory so MkdirAll fails.
	ro := filepath.Join(tmp, "ro")
	if err := os.MkdirAll(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	t.Setenv("HOME", filepath.Join(ro, "home"))

	if err := WriteTelegramToken("some:token"); err == nil {
		t.Error("WriteTelegramToken in read-only home = nil, want error")
	}
}

func TestWriteTelegramToken_CreateTempFails_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// First write successfully so ~/.claude exists.
	if err := WriteTelegramToken("first:token"); err != nil {
		t.Fatalf("setup write: %v", err)
	}
	// Make ~/.claude read-only so CreateTemp fails.
	cd := filepath.Join(tmp, ".claude")
	if err := os.Chmod(cd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cd, 0o755) })

	if err := WriteTelegramToken("second:token"); err == nil {
		t.Error("WriteTelegramToken with read-only claude dir = nil, want error")
	}
}
