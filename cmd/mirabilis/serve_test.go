//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestServeSecondInvocationIsNoop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	lock, err := tryFlock(serveLockPath(dir))
	if err != nil {
		t.Fatalf("acquire serve lock: %v", err)
	}
	defer func() { _ = lock.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, dir)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second runServe = %v, want nil", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second runServe did not return promptly — single-instance guard failed")
	}
}

func TestRegisterClientCreatesAndRemovesPidfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cleanup, err := registerClient(dir)
	if err != nil {
		t.Fatalf("registerClient: %v", err)
	}

	pid := os.Getpid()
	pidFile := filepath.Join(clientsDir(dir), strconv.Itoa(pid)+".pid")
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("pidfile not created: %v", err)
	}

	cleanup()

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("pidfile not removed after cleanup")
	}
}

func TestLiveClientCountDeadPidRemoved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cdir := clientsDir(dir)
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatal(err)
	}

	deadPid := 99999999
	deadFile := filepath.Join(cdir, strconv.Itoa(deadPid)+".pid")
	if err := os.WriteFile(deadFile, []byte(strconv.Itoa(deadPid)), 0o600); err != nil {
		t.Fatal(err)
	}

	livePid := os.Getpid()
	liveFile := filepath.Join(cdir, strconv.Itoa(livePid)+".pid")
	if err := os.WriteFile(liveFile, []byte(strconv.Itoa(livePid)), 0o600); err != nil {
		t.Fatal(err)
	}

	count, err := liveClientCount(cdir)
	if err != nil {
		t.Fatalf("liveClientCount error: %v", err)
	}
	if count != 1 {
		t.Fatalf("liveClientCount = %d, want 1", count)
	}

	if _, err := os.Stat(deadFile); !os.IsNotExist(err) {
		t.Fatal("dead pidfile not removed by liveClientCount")
	}
}

func TestLiveClientCountReadDirErrorDoesNotReap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	miraDir := filepath.Join(dir, ".mirabilis")
	if err := os.MkdirAll(miraDir, 0o755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(miraDir, "clients")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	log := &capturingLogger{}
	go func() {
		reapLoopWith(ctx, dir, log, 10*time.Millisecond, 20*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		if ctx.Err() == nil {
			t.Fatal("reapLoop exited early despite ReadDir error — must not reap on error")
		}
	case <-ctx.Done():
	}

	if !log.logged() {
		t.Error("error not logged when clients dir unreadable")
	}
}

func TestReapLoopExitsWhenNoClients(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		reapLoopWith(ctx, dir, testLogger{}, 20*time.Millisecond, 50*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("reapLoop did not exit within grace + interval timeout")
	}
}

func TestReapLoopStaysAliveWithLiveClient(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cleanup, err := registerClient(dir)
	if err != nil {
		t.Fatalf("registerClient: %v", err)
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		reapLoopWith(ctx, dir, testLogger{}, 10*time.Millisecond, 20*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		if ctx.Err() == nil {
			t.Fatal("reapLoop exited early despite live client")
		}
	case <-ctx.Done():
	}
}

func TestReapLoopReapsWhenLastClientExits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cleanup, err := registerClient(dir)
	if err != nil {
		t.Fatalf("registerClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		reapLoopWith(ctx, dir, testLogger{}, 10*time.Millisecond, 30*time.Millisecond)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cleanup()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("reapLoop did not exit after last client unregistered")
	}
}

type testLogger struct{}

func (testLogger) Error(string, ...any) {}

type capturingLogger struct {
	mu  sync.Mutex
	saw bool
}

func (c *capturingLogger) Error(_ string, _ ...any) {
	c.mu.Lock()
	c.saw = true
	c.mu.Unlock()
}

func (c *capturingLogger) logged() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saw
}
