package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const (
	nodeName = "notify"

	defaultPollInterval = 2 * time.Second
	maxTransientAttempts = 3
)

var ErrPermanent = errors.New("notify: permanent delivery failure")

func Watch(ctx context.Context, dir string, n Notifier, o *obs.Obs, interval time.Duration) {
	if interval <= 0 {
		interval = defaultPollInterval
	}
	log := o.Logger(nodeName)
	o.Set(nodeName, obs.StateOK, "")
	attempts := &attemptTracker{}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick(ctx, dir, n, o, log, attempts)
		}
	}
}

type attemptTracker struct {
	mu   sync.Mutex
	seen map[string]int
}

func (a *attemptTracker) inc(id string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.seen == nil {
		a.seen = make(map[string]int)
	}
	a.seen[id]++
	return a.seen[id]
}

func (a *attemptTracker) del(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.seen, id)
}

func tick(ctx context.Context, dir string, n Notifier, o *obs.Obs, log *slog.Logger, attempts *attemptTracker) {
	defer func() {
		if r := recover(); r != nil {
			detail := fmt.Sprintf("panic: %v", r)
			o.Set(nodeName, obs.StateDegraded, detail)
			log.Error("watcher tick panic", slog.String("detail", detail))
		}
	}()
	if err := PruneDelivered(dir, config.DeliveredRetention); err != nil {
		log.Warn("prune delivered", slog.String("error", err.Error()))
	}
	pending, err := PendingJobs(dir)
	if err != nil {
		o.Set(nodeName, obs.StateDegraded, err.Error())
		log.Warn("scan queue", slog.String("error", err.Error()))
		return
	}
	if len(pending) == 0 {
		return
	}
	var failed string
	for _, jobPath := range pending {
		if ctx.Err() != nil {
			return
		}
		if detail := deliver(ctx, dir, jobPath, n, log, attempts); detail != "" {
			failed = detail
		}
	}
	if failed != "" {
		o.Set(nodeName, obs.StateDegraded, failed)
		return
	}
	o.Set(nodeName, obs.StateOK, "")
}

func deliver(ctx context.Context, dir, jobPath string, n Notifier, log *slog.Logger, attempts *attemptTracker) string {
	job, err := ReadJob(jobPath)
	if err != nil {
		log.Warn("read job", slog.String("path", jobPath), slog.String("error", err.Error()))
		return err.Error()
	}
	sendErr := n.Send(ctx, job.ChatID, job.Text)
	if sendErr == nil {
		attempts.del(job.ID)
		status := JobStatus{ID: job.ID, OK: true, CompletedAt: time.Now().UTC()}
		if wErr := WriteStatus(dir, status); wErr != nil {
			log.Warn("write status", slog.String("job", job.ID), slog.String("error", wErr.Error()))
			return wErr.Error()
		}
		return ""
	}

	log.Warn("send", slog.String("job", job.ID), slog.String("error", sendErr.Error()))

	count := attempts.inc(job.ID)
	permanent := errors.Is(sendErr, ErrPermanent) || count >= maxTransientAttempts
	if !permanent {
		return sendErr.Error()
	}

	attempts.del(job.ID)
	status := JobStatus{
		ID:          job.ID,
		OK:          false,
		Error:       sendErr.Error(),
		CompletedAt: time.Now().UTC(),
	}
	if wErr := WriteStatus(dir, status); wErr != nil {
		log.Warn("write status", slog.String("job", job.ID), slog.String("error", wErr.Error()))
		return wErr.Error()
	}
	return sendErr.Error()
}
