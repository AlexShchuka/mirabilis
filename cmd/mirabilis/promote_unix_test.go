//go:build darwin || linux

package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func discardLog() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestPromoteLoopAcquiresAfterRelease(t *testing.T) {
	lock := filepath.Join(t.TempDir(), ".mirabilis", "mirabilis.lock")

	held, err := tryFlock(lock)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}

	var calls atomic.Int32
	got := make(chan *os.File, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		promoteLoop(ctx, lock, 5*time.Millisecond, discardLog(), func(f *os.File) {
			calls.Add(1)
			got <- f
		})
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatal("callback fired while lock was held")
	}

	_ = held.Close()

	select {
	case f := <-got:
		t.Cleanup(func() { _ = f.Close() })
	case <-time.After(2 * time.Second):
		t.Fatal("promoteLoop did not acquire after release")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("promoteLoop did not exit after acquiring")
	}

	if n := calls.Load(); n != 1 {
		t.Fatalf("callback fired %d times, want 1", n)
	}
}

func TestPromoteLoopExitsOnCtxCancel(t *testing.T) {
	lock := filepath.Join(t.TempDir(), ".mirabilis", "mirabilis.lock")

	held, err := tryFlock(lock)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	t.Cleanup(func() { _ = held.Close() })

	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		promoteLoop(ctx, lock, 5*time.Millisecond, discardLog(), func(*os.File) {
			calls.Add(1)
		})
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("promoteLoop did not exit on ctx cancel")
	}

	if n := calls.Load(); n != 0 {
		t.Fatalf("callback fired %d times after cancel, want 0 (lock never released)", n)
	}
}
