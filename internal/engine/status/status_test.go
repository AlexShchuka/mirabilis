package status

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

func newObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, err := obs.New(filepath.Join(t.TempDir(), "host.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	return o
}

func startWatcher(t *testing.T, fd *sandbox.FakeDocker, o *obs.Obs, base time.Duration) (*Watcher, context.CancelFunc) {
	t.Helper()
	w := New(fd, o)
	w.base = base
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	t.Cleanup(func() {
		cancel()
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
			t.Fatal("watcher goroutine leaked after cancel")
		}
	})
	return w, cancel
}

func eventually(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal(msg)
}

func waitState(t *testing.T, o *obs.Obs, want obs.State) obs.NodeStatus {
	t.Helper()
	var last obs.NodeStatus
	eventually(t, func() bool {
		st, ok := o.Snapshot()["container"]
		last = st
		return ok && st.State == want
	}, "container never reached state "+want.String())
	return last
}

func running(health string, started time.Time) sandbox.Container {
	return sandbox.Container{Running: true, Health: health, StartedAt: started}
}

func TestInitialRunningHealthy(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	fd := sandbox.NewFakeDocker().StubInspect(running("healthy", time.Now().Add(-2*time.Hour)), nil)
	startWatcher(t, fd, o, time.Millisecond)
	st := waitState(t, o, obs.StateOK)
	if st.Detail != "up 2h, healthy" {
		t.Fatalf("detail = %q, want %q", st.Detail, "up 2h, healthy")
	}
}

func TestInitialUnhealthy(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	fd := sandbox.NewFakeDocker().StubInspect(running("unhealthy", time.Now().Add(-5*time.Minute)), nil)
	startWatcher(t, fd, o, time.Millisecond)
	st := waitState(t, o, obs.StateDegraded)
	if st.Detail != "up 5m, unhealthy" {
		t.Fatalf("detail = %q, want %q", st.Detail, "up 5m, unhealthy")
	}
}

func TestInitialStopped(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	fd := sandbox.NewFakeDocker().StubInspect(sandbox.Container{Running: false}, nil)
	startWatcher(t, fd, o, time.Millisecond)
	st := waitState(t, o, obs.StateOff)
	if st.Detail != "not running" {
		t.Fatalf("detail = %q, want %q", st.Detail, "not running")
	}
}

func TestDaemonDown(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	fd := sandbox.NewFakeDocker().StubInspect(sandbox.Container{}, errors.New("cannot connect to the docker daemon"))
	startWatcher(t, fd, o, time.Millisecond)
	st := waitState(t, o, obs.StateOff)
	if st.Detail != "daemon unreachable" {
		t.Fatalf("detail = %q, want %q", st.Detail, "daemon unreachable")
	}
	eventually(t, func() bool { return fd.InspectCalls() >= 3 }, "watcher stopped retrying inspect")
}

func TestEventDrivenTransition(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	session := sandbox.NewFakeEvents()
	fd := sandbox.NewFakeDocker().
		StubInspect(running("healthy", time.Now()), nil).
		StubInspect(sandbox.Container{Running: false}, nil).
		StubEvents(session)
	startWatcher(t, fd, o, time.Millisecond)
	waitState(t, o, obs.StateOK)
	session.Emit("exec_create: bash")
	session.Emit("die")
	waitState(t, o, obs.StateOff)
	if got := fd.InspectCalls(); got != 2 {
		t.Fatalf("inspect calls = %d, want 2 (irrelevant event must not re-inspect)", got)
	}
	if st := o.Snapshot()["container"]; st.State != obs.StateOff {
		t.Fatalf("snapshot state = %v, want off", st.State)
	}
}

func TestResubscribeAfterEventsError(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	s1 := sandbox.NewFakeEvents()
	s2 := sandbox.NewFakeEvents()
	fd := sandbox.NewFakeDocker().
		StubInspect(running("healthy", time.Now()), nil).
		StubInspect(running("healthy", time.Now()), nil).
		StubInspect(running("unhealthy", time.Now()), nil).
		StubEvents(s1).
		StubEvents(s2)
	startWatcher(t, fd, o, time.Millisecond)
	waitState(t, o, obs.StateOK)
	s1.Fail(errors.New("event stream broken"))
	eventually(t, func() bool { return fd.EventsCalls() == 2 }, "watcher never resubscribed")
	s2.Emit("health_status: unhealthy")
	waitState(t, o, obs.StateDegraded)
}

func TestResubscribeAfterEventsClose(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	s1 := sandbox.NewFakeEvents()
	s2 := sandbox.NewFakeEvents()
	fd := sandbox.NewFakeDocker().
		StubInspect(running("healthy", time.Now()), nil).
		StubEvents(s1).
		StubEvents(s2)
	startWatcher(t, fd, o, time.Millisecond)
	waitState(t, o, obs.StateOK)
	s1.Close()
	eventually(t, func() bool { return fd.EventsCalls() == 2 }, "watcher never resubscribed after close")
}

func TestColdStart(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	fd := sandbox.NewFakeDocker().
		StubInspect(sandbox.Container{}, errors.New("daemon starting")).
		StubInspect(sandbox.Container{}, errors.New("daemon starting")).
		StubInspect(running("healthy", time.Now()), nil)
	startWatcher(t, fd, o, 20*time.Millisecond)
	waitState(t, o, obs.StateOff)
	waitState(t, o, obs.StateOK)
	if got := fd.InspectCalls(); got < 3 {
		t.Fatalf("inspect calls = %d, want >= 3", got)
	}
}

func TestCancelStopsWatcher(t *testing.T) {
	t.Parallel()
	o := newObs(t)
	fd := sandbox.NewFakeDocker().StubInspect(running("healthy", time.Now()), nil)
	w, cancel := startWatcher(t, fd, o, time.Millisecond)
	waitState(t, o, obs.StateOK)
	cancel()
	select {
	case <-w.done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop on ctx cancel")
	}
}

func TestStatusFor(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tests := []struct {
		name       string
		c          sandbox.Container
		wantState  obs.State
		wantDetail string
	}{
		{"healthy", running("healthy", now.Add(-2*time.Hour)), obs.StateOK, "up 2h, healthy"},
		{"no healthcheck", running("", now.Add(-30*time.Second)), obs.StateOK, "up <1m"},
		{"none healthcheck", running("none", now.Add(-3*24*time.Hour)), obs.StateOK, "up 3d"},
		{"starting", running("starting", now.Add(-10*time.Second)), obs.StateOK, "up <1m, health starting"},
		{"unhealthy", running("unhealthy", now.Add(-90*time.Minute)), obs.StateDegraded, "up 1h, unhealthy"},
		{"stopped", sandbox.Container{Running: false}, obs.StateOff, "not running"},
		{"zero start", running("healthy", time.Time{}), obs.StateOK, "up, healthy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st, detail := statusFor(tt.c, now)
			if st != tt.wantState || detail != tt.wantDetail {
				t.Fatalf("statusFor = (%v, %q), want (%v, %q)", st, detail, tt.wantState, tt.wantDetail)
			}
		})
	}
}
