package notify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/obs"
)

type fakeNotifier struct {
	mu       sync.Mutex
	calls    []string
	err      error
	panicMsg string
}

func (f *fakeNotifier) Send(_ context.Context, chatID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.panicMsg != "" {
		panic(f.panicMsg)
	}
	f.calls = append(f.calls, chatID+"|"+text)
	return f.err
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeNotifier) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return ""
	}
	return f.calls[len(f.calls)-1]
}

func newObs(t *testing.T) (*obs.Obs, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "obs.log")
	o, err := obs.New(logPath)
	if err != nil {
		t.Fatalf("obs.New: %v", err)
	}
	t.Cleanup(func() { o.Close() })
	return o, logPath
}

func startWatch(t *testing.T, dir string, n Notifier, o *obs.Obs) chan struct{} {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(ctx, dir, n, o, 10*time.Millisecond)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Watch did not stop after cancel")
		}
	})
	return done
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func mustWriteJob(t *testing.T, dir, id, chatID, text string) {
	t.Helper()
	if err := WriteJob(dir, Job{ID: id, ChatID: chatID, Text: text, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
}

func TestWatchDeliversPendingJobOnce(t *testing.T) {
	dir := t.TempDir()
	mustWriteJob(t, dir, "w-1", "-100", "hello")
	f := &fakeNotifier{}
	o, _ := newObs(t)
	startWatch(t, dir, f, o)

	waitFor(t, "status written", func() bool {
		_, err := ReadStatus(dir, "w-1")
		return err == nil
	})
	st, err := ReadStatus(dir, "w-1")
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if !st.OK || st.Error != "" {
		t.Errorf("status = %+v, want OK with no error", st)
	}
	if f.last() != "-100|hello" {
		t.Errorf("sent = %q, want -100|hello", f.last())
	}

	time.Sleep(100 * time.Millisecond)
	if n := f.count(); n != 1 {
		t.Errorf("send count = %d, want 1 (job must not be retried)", n)
	}
	if state := o.Snapshot()[nodeName].State; state != obs.StateOK {
		t.Errorf("obs state = %v, want ok", state)
	}
}

func TestWatchSendFailureDegradesAndNeverRetries(t *testing.T) {
	dir := t.TempDir()
	mustWriteJob(t, dir, "fail-1", "-100", "boom payload")
	f := &fakeNotifier{err: errors.New("boom")}
	o, _ := newObs(t)
	startWatch(t, dir, f, o)

	waitFor(t, "failure status written", func() bool {
		st, err := ReadStatus(dir, "fail-1")
		return err == nil && !st.OK
	})
	st, err := ReadStatus(dir, "fail-1")
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if st.OK || st.Error != "boom" {
		t.Errorf("status = %+v, want ok=false error=boom", st)
	}

	waitFor(t, "obs degraded", func() bool {
		return o.Snapshot()[nodeName].State == obs.StateDegraded
	})

	time.Sleep(100 * time.Millisecond)
	if n := f.count(); n != 1 {
		t.Errorf("send count = %d, want 1 (at-most-once on failure)", n)
	}
	if state := o.Snapshot()[nodeName].State; state != obs.StateDegraded {
		t.Errorf("obs state = %v, want degraded to persist", state)
	}
}

func TestWatchCrashWindowJobResent(t *testing.T) {
	dir := t.TempDir()
	mustWriteJob(t, dir, "crash-1", "-100", "again")
	f := &fakeNotifier{}
	o, _ := newObs(t)
	startWatch(t, dir, f, o)

	waitFor(t, "first delivery", func() bool {
		_, err := ReadStatus(dir, "crash-1")
		return err == nil && f.count() == 1
	})

	if err := os.Remove(filepath.Join(dir, "crash-1"+statusSuffix)); err != nil {
		t.Fatalf("remove status: %v", err)
	}

	waitFor(t, "re-send after status loss", func() bool {
		return f.count() == 2
	})
	waitFor(t, "status rewritten", func() bool {
		st, err := ReadStatus(dir, "crash-1")
		return err == nil && st.OK
	})
}

func TestWatchCtxCancelStopsCleanly(t *testing.T) {
	dir := t.TempDir()
	f := &fakeNotifier{}
	o, _ := newObs(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(ctx, dir, f, o, 10*time.Millisecond)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return after ctx cancel")
	}
}

func TestWatchNotifierPanicDegradesNotCrashes(t *testing.T) {
	dir := t.TempDir()
	mustWriteJob(t, dir, "panic-1", "-100", "x")
	f := &fakeNotifier{panicMsg: "kaboom"}
	o, _ := newObs(t)
	done := startWatch(t, dir, f, o)

	waitFor(t, "obs degraded by panic", func() bool {
		st := o.Snapshot()[nodeName]
		return st.State == obs.StateDegraded && strings.Contains(st.Detail, "kaboom")
	})
	select {
	case <-done:
		t.Fatal("watcher goroutine died on notifier panic")
	default:
	}
}

func TestWatchTelegramFailureTokenNeverOnDisk(t *testing.T) {
	const token = "sekret-tok-9"
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()

	dir := t.TempDir()
	mustWriteJob(t, dir, "leak-1", "-100", "x")
	n := NewTelegram(storeWithToken(t, token), srv.URL)
	o, logPath := newObs(t)
	startWatch(t, dir, n, o)

	waitFor(t, "failure status written", func() bool {
		st, err := ReadStatus(dir, "leak-1")
		return err == nil && !st.OK
	})

	rawStatus, err := os.ReadFile(filepath.Join(dir, "leak-1"+statusSuffix))
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}
	if strings.Contains(string(rawStatus), token) {
		t.Errorf("token leaked into status file: %s", rawStatus)
	}
	rawLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read obs log: %v", err)
	}
	if strings.Contains(string(rawLog), token) {
		t.Error("token leaked into obs log")
	}
	if detail := o.Snapshot()[nodeName].Detail; strings.Contains(detail, token) {
		t.Errorf("token leaked into obs detail: %s", detail)
	}
}
