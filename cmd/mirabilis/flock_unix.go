//go:build darwin || linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

var flockFile *os.File

func acquireFlock(repo string) error {
	dir := filepath.Join(repo, ".mirabilis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("flock: mkdir: %w", err)
	}
	lockPath := filepath.Join(dir, "mirabilis.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("flock: open: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return fmt.Errorf("flock: locked: %w", err)
	}
	flockFile = f
	runtime.KeepAlive(flockFile)
	return nil
}
