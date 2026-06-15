//go:build !darwin && !linux

package main

import (
	"errors"
	"os"
	"path/filepath"
)

var errFlockHeld = errors.New("flock: held by another process")

func serveLockPath(repo string) string {
	return filepath.Join(repo, ".mirabilis", "serve.lock")
}

func tryFlock(_ string) (*os.File, error) {
	return nil, errors.New("flock: not supported on this platform")
}
