//go:build darwin

package runtime

import (
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
// TODO: token source: pending isolation design (issue #115) — this is the
// single write seam; callers must not scatter keychain writes elsewhere.
func KeychainStore(name, value string) error {
	account := os.Getenv("MIRABILIS_KEYCHAIN_ACCOUNT")
	if account == "" {
		if u := os.Getenv("USER"); u != "" {
			account = u
		} else {
			account = "mirabilis"
		}
	}
	cmd := exec.Command("security", "add-generic-password",
		"-U",
		"-a", account,
		"-s", "mirabilis-"+name+"-token",
		"-w", value,
	)
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
