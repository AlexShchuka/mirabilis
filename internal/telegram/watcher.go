// Package telegram — Watcher is the host-side queue consumer.
//
// It runs on the HOST, reads job files from the bind-mounted workspace queue
// directory, delivers each via NewOutbox (enforcing the channel pin and rate
// limit), and writes a status file per job.
//
// The token is read from the host-side token file (never from container state).
// It is passed to NewOutbox and never exposed in logs or status files.
package telegram

import (
	"context"
	"fmt"
	"os"
	"time"
)

// WatcherConfig holds the parameters for RunWatcher.
type WatcherConfig struct {
	// QueueDir is the absolute path to the outbox directory on the HOST
	// filesystem (bind-mounted workspace subdir).
	QueueDir string

	// TokenPath is the path to the bot-token file on the HOST
	// (e.g. ~/.claude/.mirabilis-telegram-token, mode 0600).
	TokenPath string

	// AllowedChatID is the pinned channel ID; passed directly to NewOutbox.
	// An empty value means all sends are refused (fail closed).
	AllowedChatID string

	// PollInterval controls how often the watcher checks for new jobs.
	// Defaults to 2 seconds if zero.
	PollInterval time.Duration
}

// RunWatcher watches QueueDir for pending job files and delivers each one via
// NewOutbox. It blocks until ctx is cancelled.
//
// The channel pin enforced by NewOutbox is the only path through which a job
// can reach Telegram, so a job with a non-pinned chat_id is refused at that
// boundary and recorded as failed.
func RunWatcher(ctx context.Context, cfg WatcherConfig) error {
	if cfg.QueueDir == "" {
		return fmt.Errorf("telegram watcher: QueueDir is required")
	}
	if cfg.TokenPath == "" {
		return fmt.Errorf("telegram watcher: TokenPath is required")
	}
	interval := cfg.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	outbox, err := NewOutbox(cfg.TokenPath, cfg.AllowedChatID, alwaysConfirmWatcher)
	if err != nil {
		return fmt.Errorf("telegram watcher: %w", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := processQueue(ctx, cfg.QueueDir, outbox); err != nil {
				// Log but do not stop — the next tick will retry unprocessed jobs.
				fmt.Fprintf(os.Stderr, "[tg-watcher] WARN: %v\n", err)
			}
		}
	}
}

// alwaysConfirmWatcher is the confirm callback used by the watcher.
// The watcher operates non-interactively; jobs in the queue are pre-confirmed
// by the user having invoked tgsend --confirm inside the container.
func alwaysConfirmWatcher(_, _ string) bool { return true }

// processQueue scans for pending jobs and processes each one.
func processQueue(ctx context.Context, dir string, outbox *Outbox) error {
	pending, err := PendingJobs(dir)
	if err != nil {
		return fmt.Errorf("scan queue: %w", err)
	}
	for _, jobPath := range pending {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		processOneJob(ctx, dir, jobPath, outbox)
	}
	return nil
}

// ProcessOneJob reads, delivers, and records the status of a single job.
// It is exported (capitalised) so tests can exercise the core logic directly
// against an httptest mock Telegram server.
func ProcessOneJob(ctx context.Context, dir, jobPath string, outbox *Outbox) {
	processOneJob(ctx, dir, jobPath, outbox)
}

func processOneJob(ctx context.Context, dir, jobPath string, outbox *Outbox) {
	job, err := ReadJob(jobPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[tg-watcher] WARN: read job %q: %v\n", jobPath, err)
		return
	}

	sendErr := outbox.Send(ctx, job.ChatID, job.Text)
	status := JobStatus{
		ID:          job.ID,
		OK:          sendErr == nil,
		CompletedAt: time.Now().UTC(),
	}
	if sendErr != nil {
		// Never include the token in the error string — outbox.Send already
		// guarantees this; we record it verbatim.
		status.Error = sendErr.Error()
	}

	if err := WriteStatus(dir, status); err != nil {
		fmt.Fprintf(os.Stderr, "[tg-watcher] WARN: write status for job %q: %v\n", job.ID, err)
	}
}
