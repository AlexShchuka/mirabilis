package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func registerClient(repo string) (func(), error) {
	dir := clientsDir(repo)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return func() {}, fmt.Errorf("client: mkdir: %w", err)
	}
	pid := os.Getpid()
	path := filepath.Join(dir, strconv.Itoa(pid)+".pid")
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		return func() {}, fmt.Errorf("client: write pidfile: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil
}
