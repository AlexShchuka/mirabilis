package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const (
	nodeName = "notify"

	defaultPollInterval = 2 * time.Second
)

func Watch(ctx context.Context, dir string, n Notifier, o *obs.Obs, interval time.Duration) {
	if interval <= 0 {
		interval = defaultPollInterval
	}
	log := o.Logger(nodeName)
	o.Set(nodeName, obs.StateOK, "")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick(ctx, dir, n, o, log)
		}
	}
}

func tick(ctx context.Context, dir string, n Notifier, o *obs.Obs, log *slog.Logger) {
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
		if detail := deliver(ctx, dir, jobPath, n, log); detail != "" {
			failed = detail
		}
	}
	if failed != "" {
		o.Set(nodeName, obs.StateDegraded, failed)
		return
	}
	o.Set(nodeName, obs.StateOK, "")
}

func deliver(ctx context.Context, dir, jobPath string, n Notifier, log *slog.Logger) string {
	job, err := ReadJob(jobPath)
	if err != nil {
		log.Warn("read job", slog.String("path", jobPath), slog.String("error", err.Error()))
		return err.Error()
	}
	sendErr := n.Send(ctx, job.ChatID, job.Text)
	status := JobStatus{
		ID:          job.ID,
		OK:          sendErr == nil,
		CompletedAt: time.Now().UTC(),
	}
	if sendErr != nil {
		status.Error = sendErr.Error()
		log.Warn("send", slog.String("job", job.ID), slog.String("error", sendErr.Error()))
	}
	if err := WriteStatus(dir, status); err != nil {
		log.Warn("write status", slog.String("job", job.ID), slog.String("error", err.Error()))
		return err.Error()
	}
	if sendErr != nil {
		return sendErr.Error()
	}
	return ""
}
