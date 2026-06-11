//go:build !darwin

package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestKeychainStore_NonDarwin_Noop(t *testing.T) {
	// On non-darwin platforms KeychainStore is a no-op that always returns nil.
	if err := KeychainStore("telegram-token", "test-value"); err != nil {
		t.Errorf("KeychainStore on non-darwin = %v, want nil", err)
	}
}

func TestEnsureDocker_NotReachableNonDarwin(t *testing.T) {
	dockerShim := makeShim(t, "docker", `
case "$1" in
  info) exit 1;;
  *) exit 0;;
esac`)
	dir2 := makeShim(t, "devcontainer", `exit 0`)
	prependPath(t, dockerShim, dir2)
	err := EnsureDocker(context.Background())
	if err == nil {
		t.Fatal("EnsureDocker should error on linux when docker daemon unreachable")
	}
	if !strings.Contains(err.Error(), "systemctl start docker") || !strings.Contains(err.Error(), "WSL") {
		t.Errorf("EnsureDocker error = %q, want an actionable linux/WSL hint", err)
	}
}
