//go:build !darwin

package runtime

import (
	"context"
	"fmt"
)

func keychainLookup(_ string) (string, bool) { return "", false }

func KeychainStore(_, _ string) error { return nil }

func tryStartDocker(_ context.Context) error {
	return fmt.Errorf("docker daemon is not running — start it with 'sudo systemctl start docker', or on WSL enable Docker Desktop's WSL integration for this distro")
}
