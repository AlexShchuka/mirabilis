//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

var (
	flockMu   sync.Mutex
	flockFile *os.File
)

var errFlockHeld = errors.New("flock: held by another process")

func lockPathFor(repo string) string {
	return filepath.Join(repo, ".mirabilis", "mirabilis.lock")
}

func tryFlock(lockPath string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("flock: mkdir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("flock: open: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errFlockHeld
		}
		return nil, fmt.Errorf("flock: lock: %w", err)
	}
	return f, nil
}

func acquireFlock(repo string) error {
	f, err := tryFlock(lockPathFor(repo))
	if err != nil {
		return err
	}
	setFlock(f)
	return nil
}

func setFlock(f *os.File) {
	flockMu.Lock()
	defer flockMu.Unlock()
	flockFile = f
}

func releaseFlock() {
	flockMu.Lock()
	defer flockMu.Unlock()
	if flockFile != nil {
		_ = flockFile.Close()
		flockFile = nil
	}
}
