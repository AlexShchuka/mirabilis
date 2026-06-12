//go:build darwin

package main

import (
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

func platformStore(repo, home string) secrets.Store {
	r := exec.NewHost()
	return secrets.NewKeychainStore(r, home+"/.claude")
}
