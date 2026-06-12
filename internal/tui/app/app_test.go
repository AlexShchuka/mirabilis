package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/steps"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/app"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func init() {
	app.SetExecRunner(captureExec)
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
	mu                 sync.Mutex
	steps              []pipeline.Command
	statusCh           chan obs.Snapshot
	tokenExtracted     string
	teeCalled          bool
	saveCalls          int
	resetCalls         int
	statusUpdatesCalls int
	callLog            []string
	newTokenTeeHook    func(io.Writer, func() (string, bool))
	saveErr            error
	resetErr           error
	telegramTokens     []string
	telegramErr        error
	harnessCurrent     string
	harnessStatusErr   error
	harnessApplied     []string
	harnessApplyErr    error
	vscodeCalls        int
	vscodeErr          error
	lastHarness        string
	rememberedChoice   string
	telegramCfg        bool
	telegramMarked     bool
}

func newFakeFacade(steps []pipeline.Command) *fakeFacade {
	return &fakeFacade{
		steps:    steps,
		statusCh: make(chan obs.Snapshot, 4),
	}
}

func (f *fakeFacade) statusUpdatesCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statusUpdatesCalls
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
	f.statusUpdatesCalls++
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

func (f *fakeFacade) SaveMemory(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalls++
	f.callLog = append(f.callLog, "SaveMemory")
	return f.saveErr
}

func (f *fakeFacade) ResetSandbox(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resetCalls++
	f.callLog = append(f.callLog, "ResetSandbox")
	return f.resetErr
}

func (f *fakeFacade) ConfigureTelegram(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.telegramTokens = append(f.telegramTokens, token)
	f.callLog = append(f.callLog, "ConfigureTelegram")
	return f.telegramErr
}

func (f *fakeFacade) HarnessStatus(_ context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callLog = append(f.callLog, "HarnessStatus")
	return f.harnessCurrent, f.harnessStatusErr
}

func (f *fakeFacade) ApplyHarness(_ context.Context, choice string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.harnessApplied = append(f.harnessApplied, choice)
	f.callLog = append(f.callLog, "ApplyHarness")
	return f.harnessApplyErr
}

func (f *fakeFacade) OpenVSCode(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vscodeCalls++
	f.callLog = append(f.callLog, "OpenVSCode")
	return f.vscodeErr
}

func (f *fakeFacade) LastHarnessChoice() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastHarness
}

func (f *fakeFacade) RememberHarnessChoice(choice string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rememberedChoice = choice
	f.callLog = append(f.callLog, "RememberHarnessChoice")
	return nil
}

func (f *fakeFacade) TelegramConfigured() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.telegramCfg
}

func (f *fakeFacade) MarkTelegramConfigured() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.telegramMarked = true
	f.callLog = append(f.callLog, "MarkTelegramConfigured")
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

func TestOutboundBridge(t *testing.T) {
	step2Gate := make(chan struct{})
	step1 := &syncCommand{
		meta: pipeline.Meta{Name: "step1", Title: "Step One", Kind: pipeline.Auto},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvSpawn, Argv: []string{"docker", "run"}}
			out <- pipeline.Event{Kind: pipeline.EvLine, Line: "output line 1"}
			return nil
		},
	}
	step2 := &syncCommand{
		meta: pipeline.Meta{Name: "step2", Title: "Step Two", Kind: pipeline.Auto},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvLine, Line: "step2 line"}
			select {
			case <-step2Gate:
			case <-ctx.Done():
			}
			return nil
		},
	}

	f := newFakeFacade([]pipeline.Command{step1, step2})
	tm := newApp(t, f)

	var acc bytes.Buffer
	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, io.TeeReader(tm.Output(), &acc), func(bts []byte) bool {
		all := append(acc.Bytes(), bts...)
		return bytes.Contains(all, []byte("Step One")) &&
			bytes.Contains(all, []byte("Step Two")) &&
			bytes.Contains(all, []byte("+ docker run")) &&
			bytes.Contains(all, []byte("step2 line"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	close(step2Gate)
}

func TestInboundInteractiveCatalog(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)

	catalogStep := &syncCommand{
		meta: pipeline.Meta{Name: "catalog-step", Title: "Pick one", Kind: pipeline.Interactive},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind:    pipeline.EvWaiting,
				Step:    "catalog-step",
				Payload: catalogPayload(),
			}
			r := <-in
			resumedWith <- r
			return nil
		},
	}

	f := newFakeFacade([]pipeline.Command{catalogStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("alpha"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	var r pipeline.Result
	select {
	case r = <-resumedWith:
	case <-time.After(5 * time.Second):
		t.Fatal("Resume not called within timeout")
	}

	if r.Cancelled {
		t.Error("Resume was called with Cancelled=true, want false")
	}
}

func catalogPayload() interface{} {
	return steps.Catalog{
		Title:       "Pick one",
		Options:     []string{"alpha", "beta"},
		Selected:    nil,
		MultiSelect: true,
	}
}

func TestInboundCancelled(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)

	waitStep := &syncCommand{
		meta: pipeline.Meta{Name: "wait-step", Title: "Wait", Kind: pipeline.Interactive},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind:    pipeline.EvWaiting,
				Step:    "wait-step",
				Payload: catalogPayload(),
			}
			r := <-in
			resumedWith <- r
			if r.Cancelled {
				return pipeline.ErrCancelled
			}
			return nil
		},
	}

	f := newFakeFacade([]pipeline.Command{waitStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("alpha"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	var r pipeline.Result
	select {
	case r = <-resumedWith:
	case <-time.After(5 * time.Second):
		t.Fatal("Resume not called within timeout after esc")
	}

	if !r.Cancelled {
		t.Error("Resume was called with Cancelled=false, want true")
	}
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
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Slow Step"))
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(10*time.Millisecond))

	if time.Now().After(deadline) {
		t.Error("frame did not render within 2 seconds while slow step was running (possible UI thread block)")
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

	runDone := make(chan struct{})
	longStep := &syncCommand{
		meta: pipeline.Meta{Name: "long", Title: "Long", Kind: pipeline.Auto},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
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

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Long"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	cancel()

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Error("pipeline goroutine did not exit after ctx cancel")
	}
}

func telegramStep(resumedWith chan pipeline.Result) *syncCommand {
	return &syncCommand{
		meta:    pipeline.Meta{Name: "telegram", Title: "Telegram", Kind: pipeline.Interactive},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind:    pipeline.EvWaiting,
				Step:    "telegram",
				Payload: steps.TelegramSetup{},
			}
			r := <-in
			resumedWith <- r
			return nil
		},
	}
}

func TestTelegramWaiting_ScreenPushed(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)
	f := newFakeFacade([]pipeline.Command{telegramStep(resumedWith)})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Configure"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestTelegramWaiting_TokenResume(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)
	f := newFakeFacade([]pipeline.Command{telegramStep(resumedWith)})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Configure"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenResult{Value: "sk-ant-oat01-TESTTOKEN"})

	var r pipeline.Result
	select {
	case r = <-resumedWith:
	case <-time.After(5 * time.Second):
		t.Fatal("Resume not called within timeout")
	}
	if r.Cancelled {
		t.Error("Resume was Cancelled, want false")
	}
	if r.Value != "sk-ant-oat01-TESTTOKEN" {
		t.Errorf("Resume value = %q, want %q", r.Value, "sk-ant-oat01-TESTTOKEN")
	}
}

func TestTelegramWaiting_SkipResume(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)
	f := newFakeFacade([]pipeline.Command{telegramStep(resumedWith)})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Configure"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenResult{Value: screens.TelegramSkip})

	var r pipeline.Result
	select {
	case r = <-resumedWith:
	case <-time.After(5 * time.Second):
		t.Fatal("Resume not called within timeout")
	}
	if r.Cancelled {
		t.Error("Resume was Cancelled, want false")
	}
	if r.Value != screens.TelegramSkip {
		t.Errorf("Resume value = %v, want TelegramSkip", r.Value)
	}
}

func TestTelegramWaiting_EscCancelled(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)
	f := newFakeFacade([]pipeline.Command{telegramStep(resumedWith)})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Configure"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	var r pipeline.Result
	select {
	case r = <-resumedWith:
	case <-time.After(5 * time.Second):
		t.Fatal("Resume not called within timeout after esc")
	}
	if !r.Cancelled {
		t.Error("Resume was not Cancelled, want true")
	}
}

func TestGHAuth_ScreenAndLines(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)
	lineSent := make(chan struct{})

	ghStep := &syncCommand{
		meta:    pipeline.Meta{Name: "gh-auth", Title: "GitHub sign-in", Kind: pipeline.Interactive},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind:    pipeline.EvWaiting,
				Step:    "gh-auth",
				Payload: steps.GHAuth{Code: "ABCD-1234", URL: "https://github.com/login/device"},
			}
			<-lineSent
			out <- pipeline.Event{Kind: pipeline.EvLine, Step: "gh-auth", Line: "device-flow-progress"}
			r := <-in
			resumedWith <- r
			return nil
		},
	}

	f := newFakeFacade([]pipeline.Command{ghStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("ABCD-1234")) &&
			bytes.Contains(bts, []byte("github.com/login/device"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	close(lineSent)

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("device-flow-progress"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestReset_ConfirmCallsAndReturnsToMenu(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: screens.ActionReset})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Delete everything"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenResult{Value: true})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.NoticeResetDone))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		log := f.getCallLog()
		if len(log) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	log := f.getCallLog()
	if len(log) < 2 {
		t.Fatalf("expected 2 calls (SaveMemory, ResetSandbox), got %v", log)
	}
	if log[0] != "SaveMemory" {
		t.Errorf("first call = %q, want SaveMemory", log[0])
	}
	if log[1] != "ResetSandbox" {
		t.Errorf("second call = %q, want ResetSandbox", log[1])
	}
}

func TestReset_PopReturnsToMenuWithoutCalls(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: screens.ActionReset})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Delete everything"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenPop{})
	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 36})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("mirabilis"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	time.Sleep(100 * time.Millisecond)

	f.mu.Lock()
	save := f.saveCalls
	reset := f.resetCalls
	f.mu.Unlock()

	if save != 0 || reset != 0 {
		t.Errorf("expected no facade calls on ScreenPop, got saveCalls=%d resetCalls=%d", save, reset)
	}
}

func TestReset_SuccessNoticeAppearsAfterCompletion(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: screens.ActionReset})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Delete everything"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenResult{Value: true})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.NoticeResetDone))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestReset_FailureShowsFailureNotice(t *testing.T) {
	f := newFakeFacade(nil)
	f.resetErr = errors.New("compose down failed")
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: screens.ActionReset})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Delete everything"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenResult{Value: true})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.NoticeResetFailed))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	f.mu.Lock()
	saveN := f.saveCalls
	resetN := f.resetCalls
	f.mu.Unlock()
	if saveN != 1 {
		t.Errorf("SaveMemory called %d times, want 1", saveN)
	}
	if resetN != 1 {
		t.Errorf("ResetSandbox called %d times, want 1", resetN)
	}
}

func TestStatusUpdatesSubscribedOnce(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	for i := range 5 {
		f.mu.Lock()
		f.statusCh <- obs.Snapshot{"node": obs.NodeStatus{State: obs.StateOK, Detail: strconv.Itoa(i)}}
		f.mu.Unlock()
	}

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("mirabilis"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	time.Sleep(100 * time.Millisecond)

	n := f.statusUpdatesCount()
	if n != 1 {
		t.Errorf("StatusUpdates() called %d times, want exactly 1 (subscribe-once violation)", n)
	}
}
