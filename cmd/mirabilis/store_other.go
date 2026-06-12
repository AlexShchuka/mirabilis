//go:build !darwin

package main

import (
	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

func platformStore(repo, home string) secrets.Store {
	return secrets.NewFileStore(home + "/.claude")
}
