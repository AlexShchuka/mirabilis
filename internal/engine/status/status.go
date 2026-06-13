package status

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

var _ sandbox.Docker = (*sandbox.Moby)(nil)

const (
	node     = "container"
	maxShift = 5
)

type Watcher struct {
	docker sandbox.Docker
	obs    *obs.Obs
	done   chan struct{}
	base   time.Duration
	max    time.Duration
}

func New(d sandbox.Docker, o *obs.Obs) *Watcher {
	return &Watcher{
		docker: d,
		obs:    o,
		done:   make(chan struct{}),
		base:   time.Second,
		max:    30 * time.Second,
	}
}

func (w *Watcher) Start(ctx context.Context) {
	go w.run(ctx)
}

func (w *Watcher) run(ctx context.Context) {
	defer close(w.done)
	failures := 0
	for {
		c, err := w.docker.Inspect(ctx)
		if err != nil {
			w.obs.Set(node, obs.StateOff, "daemon unreachable")
			failures++
			if !w.sleep(ctx, failures) {
				return
			}
			continue
		}
		failures = 0
		w.report(c)
		events, errs := w.docker.Events(ctx)
		stop, processed := w.consume(ctx, events, errs)
		if stop {
			return
		}
		if processed {
			failures = 0
		}
		failures++
		if !w.sleep(ctx, failures) {
			return
		}
	}
}

func (w *Watcher) consume(ctx context.Context, events <-chan sandbox.ContainerEvent, errs <-chan error) (stop, processed bool) {
	for {
		select {
		case <-ctx.Done():
			return true, processed
		case <-errs:
			return false, processed
		case ev, ok := <-events:
			if !ok {
				return false, processed
			}
			if !relevant(ev.Action) {
				continue
			}
			c, err := w.docker.Inspect(ctx)
			if err != nil {
				w.obs.Set(node, obs.StateOff, "daemon unreachable")
				return false, processed
			}
			processed = true
			w.report(c)
		}
	}
}

func (w *Watcher) report(c sandbox.Container) {
	st, detail := statusFor(c, time.Now())
	w.obs.Set(node, st, detail)
}

func (w *Watcher) sleep(ctx context.Context, failures int) bool {
	shift := failures - 1
	if shift > maxShift {
		shift = maxShift
	}
	d := w.base << shift
	if d > w.max {
		d = w.max
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func relevant(action string) bool {
	if strings.HasPrefix(action, "health_status") {
		return true
	}
	switch action {
	case "create", "start", "restart", "pause", "unpause", "stop", "kill", "die", "oom", "destroy":
		return true
	}
	return false
}

func statusFor(c sandbox.Container, now time.Time) (obs.State, string) {
	if !c.Running {
		return obs.StateOff, "not running"
	}
	up := uptime(c.StartedAt, now)
	switch c.Health {
	case "unhealthy":
		return obs.StateDegraded, up + ", unhealthy"
	case "starting":
		return obs.StateOK, up + ", health starting"
	case "healthy":
		return obs.StateOK, up + ", healthy"
	default:
		return obs.StateOK, up
	}
}

func uptime(started, now time.Time) string {
	if started.IsZero() || now.Before(started) {
		return "up"
	}
	d := now.Sub(started)
	switch {
	case d < time.Minute:
		return "up <1m"
	case d < time.Hour:
		return fmt.Sprintf("up %dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("up %dh", int(d.Hours()))
	default:
		return fmt.Sprintf("up %dd", int(d.Hours())/24)
	}
}
