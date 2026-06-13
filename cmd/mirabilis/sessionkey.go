package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func sessionKeyPath(repo string) string {
	return filepath.Join(repo, ".mirabilis", "session-key")
}

func writeSessionKey(repo, key string) error {
	dir := filepath.Join(repo, ".mirabilis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("session-key: mkdir: %w", err)
	}
	path := sessionKeyPath(repo)
	if err := os.WriteFile(path, []byte(key), 0o600); err != nil {
		return fmt.Errorf("session-key: write: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("session-key: chmod: %w", err)
	}
	return nil
}

func readSessionKey(repo string) (string, error) {
	data, err := os.ReadFile(sessionKeyPath(repo))
	if err != nil {
		return "", fmt.Errorf("session-key: read: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
