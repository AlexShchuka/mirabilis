package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
)

const (
	reapInterval  = 5 * time.Second
	reapGrace     = 30 * time.Second
	clientsSubdir = "clients"
)

func clientsDir(repo string) string {
	return filepath.Join(repo, ".mirabilis", clientsSubdir)
}

func runServe(ctx context.Context, repo string) error {
	lock, err := tryFlock(serveLockPath(repo))
	if err != nil {
		if errors.Is(err, errFlockHeld) {
			return nil
		}
		return fmt.Errorf("serve: lock: %w", err)
	}
	defer func() { _ = lock.Close() }()

	f, err := newFacade(repo)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	defer f.obs.Close()

	proxy := f.newProxy("")
	if err := writeSessionKey(repo, proxy.Key()); err != nil {
		f.obs.Logger("serve").Error("session-key persist failed", "err", err)
	}
	go func() {
		if err := proxy.Start(ctx); err != nil {
			f.obs.Logger("serve").Error("proxy listen failed", "err", err)
		}
	}()
	go notify.Watch(ctx, notify.OutboxDir(repo), notify.NewTelegram(f.store, ""), f.obs, 0)

	reapLoop(ctx, repo, f.obs.Logger("serve"))
	return nil
}

func reapLoop(ctx context.Context, repo string, log interface{ Error(string, ...any) }) {
	reapLoopWith(ctx, repo, log, reapInterval, reapGrace)
}

func reapLoopWith(ctx context.Context, repo string, log interface{ Error(string, ...any) }, interval, grace time.Duration) {
	dir := clientsDir(repo)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var zeroSince time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := liveClientCount(dir)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Error("clients dir unreadable — skipping reap", "err", err)
				zeroSince = time.Time{}
				continue
			}
			switch {
			case n > 0:
				zeroSince = time.Time{}
			case zeroSince.IsZero():
				zeroSince = time.Now()
			case time.Since(zeroSince) >= grace:
				return
			}
		}
	}
}

func liveClientCount(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	live := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pid") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			_ = os.Remove(path)
			continue
		}
		if pidAlive(pid) {
			live++
		} else {
			_ = os.Remove(path)
		}
	}
	return live, nil
}

func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
