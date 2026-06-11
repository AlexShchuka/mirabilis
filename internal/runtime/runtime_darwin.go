//go:build darwin

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func keychainLookup(name string) (string, bool) {
	account := os.Getenv("MIRABILIS_KEYCHAIN_ACCOUNT")
	if account == "" {
		if u := os.Getenv("USER"); u != "" {
			account = u
		} else {
			account = "mirabilis"
		}
	}
	out, err := exec.Command("security", "find-generic-password", "-a", account, "-s", "mirabilis-"+name+"-token", "-w").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// KeychainStore writes value under name into the macOS Keychain.
// An existing entry for the same service+account is updated atomically via
// add-generic-password -U (update if exists).
//
// SEC-3: the secret is fed via STDIN. `security` reads the password from
// stdin when `-w` is given with NO value argument, so the token never appears
// in argv (not visible in `ps`). NOTE: this path is macOS-only and cannot run
// in the Linux dev container / CI — it is verified by cross-compile (GOOS=darwin)
// and by inspection; the live keychain write must be confirmed on a real Mac.
// (An earlier form passed `-w /dev/stdin`, which `security` stores as the
// literal string "/dev/stdin" — wrong; do not reintroduce it.)
//
// This is the single keychain-write seam (issue #115); callers must not
// scatter keychain writes elsewhere.
func KeychainStore(name, value string) error {
	if os.Getenv("CI") != "" {
		return nil
	}
	account := os.Getenv("MIRABILIS_KEYCHAIN_ACCOUNT")
	if account == "" {
		if u := os.Getenv("USER"); u != "" {
			account = u
		} else {
			account = "mirabilis"
		}
	}
	// Pass the password via stdin, not as a -w <value> argument, to avoid
	// argv exposure in `ps`. `-w` is the LAST flag with no value, so `security`
	// reads the password from stdin (a pipe here, not a tty).
	cmd := exec.Command("security", "add-generic-password",
		"-U",
		"-a", account,
		"-s", "mirabilis-"+name+"-token",
		"-w",
	)
	cmd.Stdin = bytes.NewBufferString(value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain store %q: %w (%s)", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func tryStartDocker(_ context.Context) error {
	_ = exec.Command("open", "-a", "Docker").Run()
	for i := 0; i < 60; i++ {
		if dockerReachable() {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("docker did not come up — open Docker Desktop and run mirabilis again")
}
