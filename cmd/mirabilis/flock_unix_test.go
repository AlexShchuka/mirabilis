//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTryFlockHeldReturnsSentinel(t *testing.T) {
	lock := filepath.Join(t.TempDir(), ".mirabilis", "mirabilis.lock")

	f1, err := tryFlock(lock)
	if err != nil {
		t.Fatalf("first tryFlock: %v", err)
	}
	t.Cleanup(func() { _ = f1.Close() })

	_, err = tryFlock(lock)
	if !errors.Is(err, errFlockHeld) {
		t.Fatalf("second tryFlock err = %v, want errFlockHeld", err)
	}

	_ = f1.Close()
	f2, err := tryFlock(lock)
	if err != nil {
		t.Fatalf("tryFlock after release: %v", err)
	}
	_ = f2.Close()
}

func TestTryFlockRealErrorNotSentinel(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(blocker, "child", "mirabilis.lock")
	_, err := tryFlock(lock)
	if err == nil {
		t.Fatal("tryFlock with file in the dir path = nil, want error")
	}
	if errors.Is(err, errFlockHeld) {
		t.Fatalf("real mkdir error misclassified as errFlockHeld: %v", err)
	}
}
