//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"
)

const promoteInterval = time.Second

func promoteLoop(ctx context.Context, lockPath string, interval time.Duration, log *slog.Logger, onAcquire func(*os.File)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			f, err := tryFlock(lockPath)
			if err == nil {
				onAcquire(f)
				return
			}
			if !errors.Is(err, errFlockHeld) {
				log.Error("promotion: flock poll failed", "err", err)
			}
		}
	}
}
