package obs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestObs(t *testing.T) *Obs {
	t.Helper()
	o, err := New(filepath.Join(t.TempDir(), "obs.log"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = o.Close() })
	return o
}

func TestStateString(t *testing.T) {
	tests := []struct {
		want string
		st   State
	}{
		{want: "unknown", st: StateUnknown},
		{want: "ok", st: StateOK},
		{want: "degraded", st: StateDegraded},
		{want: "off", st: StateOff},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.st.String(); got != tt.want {
				t.Errorf("State(%d).String() = %q, want %q", int(tt.st), got, tt.want)
			}
		})
	}
}

func TestSetSnapshotRoundtrip(t *testing.T) {
	o := newTestObs(t)
	o.Set("docker", StateOK, "up")
	o.Set("notify", StateDegraded, "timeout")

	snap := o.Snapshot()
	wantDocker := NodeStatus{Detail: "up", State: StateOK}
	if got := snap["docker"]; got != wantDocker {
		t.Fatalf("snap[docker] = %+v, want %+v", got, wantDocker)
	}
	wantNotify := NodeStatus{Detail: "timeout", State: StateDegraded}
	if got := snap["notify"]; got != wantNotify {
		t.Fatalf("snap[notify] = %+v, want %+v", got, wantNotify)
	}

	snap["docker"] = NodeStatus{Detail: "mutated", State: StateOff}
	delete(snap, "notify")
	again := o.Snapshot()
	if got := again["docker"]; got != wantDocker {
		t.Errorf("after mutation snap[docker] = %+v, want %+v", got, wantDocker)
	}
	if got := again["notify"]; got != wantNotify {
		t.Errorf("after mutation snap[notify] = %+v, want %+v", got, wantNotify)
	}
}

func TestConcurrentSet(t *testing.T) {
	o := newTestObs(t)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			node := strconv.Itoa(i)
			for j := range 100 {
				o.Set(node, StateOK, strconv.Itoa(j))
			}
		})
	}
	wg.Wait()

	snap := o.Snapshot()
	if len(snap) != 10 {
		t.Fatalf("len(snap) = %d, want 10", len(snap))
	}
	want := NodeStatus{Detail: "99", State: StateOK}
	for i := range 10 {
		if got := snap[strconv.Itoa(i)]; got != want {
			t.Errorf("snap[%d] = %+v, want %+v", i, got, want)
		}
	}
}

func TestWatchLatestWins(t *testing.T) {
	o := newTestObs(t)
	first := o.Watch()
	second := o.Watch()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 50 {
			o.Set("docker", StateDegraded, strconv.Itoa(i))
		}
		o.Set("docker", StateOK, "final")
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Set blocked while watcher was not reading")
	}

	want := NodeStatus{Detail: "final", State: StateOK}
	for i, ch := range []<-chan Snapshot{first, second} {
		select {
		case snap := <-ch:
			if got := snap["docker"]; got != want {
				t.Errorf("watcher %d got %+v, want %+v", i, got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("watcher %d got no snapshot", i)
		}
	}
}

func TestLoggerWritesNodeAttr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obs.log")
	o, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = o.Close() })

	o.Logger("docker").Info("container up")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	out := string(data)
	for _, want := range []string{`msg="container up"`, "node=docker", "level=INFO"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}

func TestNewCreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "logs")
	path := filepath.Join(dir, "obs.log")
	o, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = o.Close() })

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", dir)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("dir perm = %v, want 0700", got)
	}
	finfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat file: %v", err)
	}
	if got := finfo.Mode().Perm(); got != 0o600 {
		t.Errorf("file perm = %v, want 0600", got)
	}
}

func TestClose(t *testing.T) {
	o, err := New(filepath.Join(t.TempDir(), "obs.log"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := o.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := o.Close(); err == nil {
		t.Error("second Close returned nil, want error")
	}
}
