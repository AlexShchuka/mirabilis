//go:build !darwin

package runtime

import (
	"context"
	"fmt"
)

func keychainLookup(_ string) (string, bool) { return "", false }

func tryStartDocker(_ context.Context) error {
	return fmt.Errorf("docker daemon is not running")
}
