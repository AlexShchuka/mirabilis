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
