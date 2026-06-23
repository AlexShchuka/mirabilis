package app_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/app"
)

func TestMain(m *testing.M) {
	app.SetExecRunner(captureExec)
	m.Run()
}

var (
	execMu      sync.Mutex
	capturedCmd tea.ExecCommand
	execCb      tea.ExecCallback
)

func captureExec(c tea.ExecCommand, fn tea.ExecCallback) tea.Cmd {
	execMu.Lock()
	capturedCmd = c
	execCb = fn
	execMu.Unlock()
	return nil
}

func getCaptured() (tea.ExecCommand, tea.ExecCallback) {
	execMu.Lock()
	defer execMu.Unlock()
	return capturedCmd, execCb
}

func resetCaptured() {
	execMu.Lock()
	capturedCmd, execCb = nil, nil
	execMu.Unlock()
}

type fakeFacade struct {
	mu              sync.Mutex
	steps           []pipeline.Command
	statusCh        chan obs.Snapshot
	tokenExtracted  string
	teeCalled       bool
	callLog         []string
	newTokenTeeHook func(io.Writer, func() (string, bool))
}

func newFakeFacade(steps []pipeline.Command) *fakeFacade {
	return &fakeFacade{
		steps:    steps,
		statusCh: make(chan obs.Snapshot, 4),
	}
}

func (f *fakeFacade) LaunchSteps() []pipeline.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callLog = append(f.callLog, "LaunchSteps")
	return f.steps
}

func (f *fakeFacade) Version() string { return "vtest" }

func (f *fakeFacade) Logger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func (f *fakeFacade) StatusUpdates() <-chan obs.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statusCh
}

func (f *fakeFacade) OnTokenExtracted(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenExtracted = token
}

func (f *fakeFacade) NewTokenTee() (io.Writer, func() (string, bool)) {
	f.mu.Lock()
	hook := f.newTokenTeeHook
	f.teeCalled = true
	f.mu.Unlock()
	buf := &bytes.Buffer{}
	get := func() (string, bool) {
		s := buf.String()
		return s, s != ""
	}
	if hook != nil {
		hook(buf, get)
	}
	return buf, get
}

func (f *fakeFacade) logCall(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callLog = append(f.callLog, name)
}

func (f *fakeFacade) SaveMemory(_ context.Context) error {
	f.logCall("SaveMemory")
	return nil
}

func (f *fakeFacade) ResetSandbox(_ context.Context) error {
	f.logCall("ResetSandbox")
	return nil
}

func (f *fakeFacade) HarnessStatus(_ context.Context) (string, error) {
	f.logCall("HarnessStatus")
	return "", nil
}

func (f *fakeFacade) ApplyHarness(_ context.Context, _ string) error {
	f.logCall("ApplyHarness")
	return nil
}

func (f *fakeFacade) OpenVSCode(_ context.Context) error {
	f.logCall("OpenVSCode")
	return nil
}

func (f *fakeFacade) OpenURL(_ context.Context, _ string) error {
	f.logCall("OpenURL")
	return nil
}

func (f *fakeFacade) CopyText(_ context.Context, _ string) error {
	f.logCall("CopyText")
	return nil
}

func (f *fakeFacade) LastHarnessChoice() string { return "" }

func (f *fakeFacade) RememberHarnessChoice(_ string) error {
	f.logCall("RememberHarnessChoice")
	return nil
}

func (f *fakeFacade) getCallLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.callLog))
	copy(out, f.callLog)
	return out
}

type syncCommand struct {
	meta    pipeline.Meta
	checkFn func(context.Context) (bool, error)
	runFn   func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error
}

func (c *syncCommand) Meta() pipeline.Meta { return c.meta }
func (c *syncCommand) Check(ctx context.Context) (bool, error) {
	if c.checkFn != nil {
		return c.checkFn(ctx)
	}
	return false, nil
}
func (c *syncCommand) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	if c.runFn != nil {
		return c.runFn(ctx, out, in)
	}
	return nil
}

func newApp(t *testing.T, f *fakeFacade) *teatest.TestModel {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	a := app.New(context.Background(), f)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })
	return tm
}

func TestLatencyGolden(t *testing.T) {
	started := make(chan struct{})
	slowStep := &syncCommand{
		meta: pipeline.Meta{Name: "slow", Title: "Slow Step", Kind: pipeline.Auto},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			close(started)
			select {
			case <-time.After(10 * time.Second):
			case <-ctx.Done():
			}
			return ctx.Err()
		},
	}

	f := newFakeFacade([]pipeline.Command{slowStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("slow step did not start")
	}

	deadline := time.Now().Add(2 * time.Second)
	if err := tm.Quit(); err != nil {
		t.Fatalf("quit failed: %v", err)
	}

	if time.Now().After(deadline) {
		t.Error("UI did not respond to quit within 2 seconds while slow step was running (possible UI thread block)")
	}
}

func TestTerminalClaudeAuth(t *testing.T) {
	var tokenTee io.Writer
	var tokenGetFn func() (string, bool)
	var teeReady = make(chan struct{})

	f := newFakeFacade(nil)
	f.newTokenTeeHook = func(w io.Writer, get func() (string, bool)) {
		tokenTee = w
		tokenGetFn = get
		close(teeReady)
	}

	claudeAuthStep := &syncCommand{
		meta:    pipeline.Meta{Name: "claude-auth", Title: "Claude auth", Kind: pipeline.Terminal},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind: pipeline.EvWaiting,
				Step: "claude-auth",
				Argv: []string{"claude", "setup-token"},
			}
			r := <-in
			if r.Cancelled {
				return pipeline.ErrCancelled
			}
			return nil
		},
	}

	f.steps = []pipeline.Command{claudeAuthStep}
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	select {
	case <-teeReady:
	case <-time.After(5 * time.Second):
		t.Fatal("NewTokenTee was not called within timeout")
	}

	_, _ = tokenTee.Write([]byte("sk-ant-oat01-TESTTOKEN"))
	_ = tokenGetFn

	var cmd tea.ExecCommand
	var cb tea.ExecCallback
	captureDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(captureDeadline) {
		cmd, cb = getCaptured()
		if cmd != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if cmd == nil {
		t.Fatal("execRunner was not called")
	}
	if cb == nil {
		t.Fatal("execRunner callback is nil")
	}

	cb(nil)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		tok := f.tokenExtracted
		f.mu.Unlock()
		if tok == "sk-ant-oat01-TESTTOKEN" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("OnTokenExtracted not called with expected token; got %q", f.tokenExtracted)
}

func TestNoLeakOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	started := make(chan struct{})
	runDone := make(chan struct{})
	longStep := &syncCommand{
		meta: pipeline.Meta{Name: "long", Title: "Long", Kind: pipeline.Auto},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			close(started)
			defer close(runDone)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	f := newFakeFacade([]pipeline.Command{longStep})
	a := app.New(ctx, f)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() { _ = tm.Quit() })

	tm.Send(bus.MenuChosen{Action: "launch"})

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("long step did not start")
	}

	cancel()

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Error("pipeline goroutine did not exit after ctx cancel")
	}
}
