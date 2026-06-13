//go:build integration && (darwin || linux)

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mirabilis")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func waitForLock(t *testing.T, repo string, deadline time.Duration) {
	t.Helper()
	lock := filepath.Join(repo, ".mirabilis", "mirabilis.lock")
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if _, err := os.Stat(lock); err == nil {
			f, ferr := tryFlock(lock)
			if ferr == errFlockHeld {
				return
			}
			if ferr == nil {
				_ = f.Close()
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("flock never taken by the child process")
}

func TestSIGHUPExitsAndReleasesFlock(t *testing.T) {
	bin := buildBinary(t)
	repo := t.TempDir()

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "MIRABILIS_REPO="+repo, "MIRABILIS_VERSION=test")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	t.Cleanup(func() { _ = ptmx.Close() })

	waitForLock(t, repo, 10*time.Second)

	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("signal SIGHUP: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("process did not exit within deadline after SIGHUP")
	}

	lock := filepath.Join(repo, ".mirabilis", "mirabilis.lock")
	f, ferr := tryFlock(lock)
	if ferr != nil {
		t.Fatalf("flock not reacquirable after child exit: %v", ferr)
	}
	_ = f.Close()
}
