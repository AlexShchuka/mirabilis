//go:build !darwin

package runtime

import (
	"context"
	"fmt"
)

func keychainLookup(_ string) (string, bool) { return "", false }

// KeychainStore is a no-op on non-macOS. Callers should use the file-based
// fallback (WriteTelegramToken writes to ~/.claude/.mirabilis-telegram-token).
// TODO: token source: pending isolation design (issue #115).
func KeychainStore(_, _ string) error { return nil }

func tryStartDocker(_ context.Context) error {
	return fmt.Errorf("docker daemon is not running — start it with 'sudo systemctl start docker', or on WSL enable Docker Desktop's WSL integration for this distro")
}
