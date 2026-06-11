package provision

import (
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/runtime"
)

const (
	// FileTelegramToken is the statefile name for the Telegram bot token.
	// Stored in ~/.claude/ with mode 0600 on the HOST — never git-tracked,
	// never in the image, never in any workspace path that the agent can
	// reach as a context file.
	FileTelegramToken = ".mirabilis-telegram-token"
)

// TelegramTokenPath returns the path where the Telegram bot token is persisted
// on the host. The file has mode 0600 and is outside every git-tracked path.
func TelegramTokenPath() string {
	return filepath.Join(claudeDir(), FileTelegramToken)
}

// WriteTelegramToken is the single write seam for the bot token.
// TODO: token source: pending isolation design (issue #115) — this function
// writes to both the macOS Keychain (best-effort on darwin) AND the host-side
// file at ~/.claude/.mirabilis-telegram-token (0600). The keychain write is
// attempted first; if it fails, only the file is used. keychainGet reads
// keychain → env → file in priority order, so either backend works.
//
// The token is never written to any git-tracked file or workspace volume.
// The container does NOT receive the token via env or file — see ComposeEnv.
func WriteTelegramToken(token string) error {
	// Best-effort keychain write (darwin: security add-generic-password -U;
	// other platforms: no-op returning nil). We intentionally do NOT fail if
	// the keychain write succeeds, because the file provides the fallback path
	// and keychainGet checks keychain before env before file.
	_ = runtime.KeychainStore("telegram-token", token)

	// Always write the host-side file as the authoritative fallback.
	return writeTokenFile(token)
}

// writeTokenFile atomically writes token to TelegramTokenPath() with mode 0600.
func writeTokenFile(token string) error {
	cd := claudeDir()
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return err
	}
	dest := TelegramTokenPath()
	tmp, err := os.CreateTemp(cd, ".tgtoken-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(token + "\n"); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dest)
}

// ReadTelegramTokenFile reads the bot token from the host-side statefile.
// Returns empty string if the file does not exist.
func ReadTelegramTokenFile() string {
	data, err := os.ReadFile(TelegramTokenPath())
	if err != nil {
		return ""
	}
	// trim trailing newline written by WriteTelegramToken
	s := string(data)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
