package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

type stubFacade struct {
	mu               sync.Mutex
	steps            []pipeline.Command
	launchCalls      int
	telegramTokens   []string
	telegramErr      error
	harnessCurrent   string
	harnessStatusErr error
	harnessApplied   []string
	harnessApplyErr  error
	vscodeCalls      int
	vscodeErr        error
	saveCalls        int
	resetCalls       int
}

func (f *stubFacade) LaunchSteps() []pipeline.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.launchCalls++
	return f.steps
}

func (f *stubFacade) Logger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func (f *stubFacade) StatusUpdates() <-chan obs.Snapshot { return make(chan obs.Snapshot, 1) }

func (f *stubFacade) OnTokenExtracted(string) {}

func (f *stubFacade) NewTokenTee() (io.Writer, func() (string, bool)) {
	return io.Discard, func() (string, bool) { return "", false }
}

func (f *stubFacade) SaveMemory(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalls++
	return nil
}

func (f *stubFacade) ResetSandbox(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resetCalls++
	return nil
}

func (f *stubFacade) ConfigureTelegram(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.telegramTokens = append(f.telegramTokens, token)
	return f.telegramErr
}

func (f *stubFacade) HarnessStatus(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.harnessCurrent, f.harnessStatusErr
}

func (f *stubFacade) ApplyHarness(_ context.Context, choice string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.harnessApplied = append(f.harnessApplied, choice)
	return f.harnessApplyErr
}

func (f *stubFacade) OpenVSCode(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vscodeCalls++
	return f.vscodeErr
}

func (f *stubFacade) launches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.launchCalls
}

type stateStep struct {
	meta  pipeline.Meta
	runFn func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error
}

func (c *stateStep) Meta() pipeline.Meta                 { return c.meta }
func (c *stateStep) Check(context.Context) (bool, error) { return false, nil }
func (c *stateStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	if c.runFn != nil {
		return c.runFn(ctx, out, in)
	}
	return nil
}

func newStateApp(t *testing.T, f Facade) App {
	t.Helper()
	a := New(context.Background(), f)
	t.Cleanup(a.cancel)
	return a
}

func step(t *testing.T, a App, msg tea.Msg) (App, tea.Cmd) {
	t.Helper()
	m, cmd := a.Update(msg)
	na, ok := m.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", m)
	}
	return na, cmd
}

func runMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	return cmd()
}

func nextEvent(t *testing.T, p *pipeline.Pipeline) (pipeline.Event, bool) {
	t.Helper()
	select {
	case ev, ok := <-p.Events():
		return ev, ok
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a pipeline event")
	}
	return pipeline.Event{}, false
}

func driveUntil(t *testing.T, a App, stop func(App, pipeline.Event) bool) App {
	t.Helper()
	p := a.pipe
	if p == nil {
		t.Fatal("no pipeline running")
	}
	for {
		ev, ok := nextEvent(t, p)
		if !ok {
			a, _ = step(t, a, pipelineDoneMsg{})
			return a
		}
		a, _ = step(t, a, pipelineEventMsg{ev: ev})
		if stop(a, ev) {
			return a
		}
	}
}

func driveUntilDone(t *testing.T, a App) App {
	t.Helper()
	return driveUntil(t, a, func(_ App, ev pipeline.Event) bool {
		return ev.Kind == pipeline.EvPipelineDone
	})
}

func driveUntilWaiting(t *testing.T, a App) App {
	t.Helper()
	return driveUntil(t, a, func(a App, _ pipeline.Event) bool {
		return a.waiting != ""
	})
}

func menuNotice(t *testing.T, a App) string {
	t.Helper()
	m, ok := a.router.Top().(screens.Menu)
	if !ok {
		t.Fatalf("top screen is %T, want screens.Menu", a.router.Top())
	}
	return m.Notice()
}

func waitingStep(name string, payload any, resumed chan pipeline.Result) *stateStep {
	return &stateStep{
		meta: pipeline.Meta{Name: name, Title: name, Kind: pipeline.Interactive},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: name, Payload: payload}
			r := <-in
			if resumed != nil {
				resumed <- r
			}
			if r.Cancelled {
				return pipeline.ErrCancelled
			}
			return nil
		},
	}
}

func TestStateTelegramConfigure(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionTelegram})
	if a.menuAction != "telegram" {
		t.Fatalf("menuAction = %q, want telegram", a.menuAction)
	}

	a, cmd := step(t, a, bus.ScreenResult{Value: "123456:bot-token"})
	if !a.busy {
		t.Error("busy = false while telegram configure runs, want true")
	}
	a, _ = step(t, a, runMsg(t, cmd))
	if a.busy {
		t.Error("busy = true after telegram done, want false")
	}
	if got := menuNotice(t, a); got != uistr.NoticeTelegramDone {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeTelegramDone)
	}
	if len(f.telegramTokens) != 1 || f.telegramTokens[0] != "123456:bot-token" {
		t.Errorf("ConfigureTelegram tokens = %v", f.telegramTokens)
	}
}

func TestStateTelegramSkip(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionTelegram})
	a, _ = step(t, a, bus.ScreenResult{Value: screens.TelegramSkip})
	if a.menuAction != "" {
		t.Errorf("menuAction = %q after skip, want empty", a.menuAction)
	}
	if a.busy {
		t.Error("busy = true after skip, want false")
	}
	if len(f.telegramTokens) != 0 {
		t.Errorf("ConfigureTelegram called on skip: %v", f.telegramTokens)
	}
}

func TestStateTelegramFailure(t *testing.T) {
	f := &stubFacade{telegramErr: errors.New("boom: channel not detected")}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionTelegram})
	a, cmd := step(t, a, bus.ScreenResult{Value: "123456:bot-token"})
	a, _ = step(t, a, runMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeTelegramErr+"boom: channel not detected" {
		t.Errorf("notice = %q", got)
	}
}

func TestStateHarnessFlow(t *testing.T) {
	f := &stubFacade{harnessCurrent: screens.HarnessOff}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	if !a.busy {
		t.Error("busy = false while harness status runs, want true")
	}
	a, _ = step(t, a, runMsg(t, cmd))
	if a.busy {
		t.Error("busy = true after status arrived, want false")
	}
	if a.menuAction != "harness" {
		t.Fatalf("menuAction = %q, want harness", a.menuAction)
	}
	if a.router.Depth() != 2 {
		t.Fatalf("router depth = %d after harness screen push, want 2", a.router.Depth())
	}

	a, cmd = step(t, a, bus.ScreenResult{Value: screens.HarnessReinstall})
	if !a.busy {
		t.Error("busy = false while harness apply runs, want true")
	}
	a, _ = step(t, a, runMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeHarnessDone {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeHarnessDone)
	}
	if len(f.harnessApplied) != 1 || f.harnessApplied[0] != screens.HarnessReinstall {
		t.Errorf("ApplyHarness = %v, want [reinstall]", f.harnessApplied)
	}
}

func TestStateHarnessStatusError(t *testing.T) {
	f := &stubFacade{harnessStatusErr: errors.New("container claude unavailable")}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	a, _ = step(t, a, runMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeHarnessErr+"container claude unavailable" {
		t.Errorf("notice = %q", got)
	}
	if len(f.harnessApplied) != 0 {
		t.Errorf("ApplyHarness called after status error: %v", f.harnessApplied)
	}
}

func TestStateHarnessApplyError(t *testing.T) {
	f := &stubFacade{harnessCurrent: "missing", harnessApplyErr: errors.New("plugin install failed")}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	a, _ = step(t, a, runMsg(t, cmd))
	a, cmd = step(t, a, bus.ScreenResult{Value: screens.HarnessOn})
	a, _ = step(t, a, runMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeHarnessErr+"plugin install failed" {
		t.Errorf("notice = %q", got)
	}
	if a.busy {
		t.Error("busy = true after harness apply error, want false")
	}
}

func TestStateVSCode(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	if got := menuNotice(t, a); got != uistr.NoticeVSCodeOpening {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeVSCodeOpening)
	}
	a, _ = step(t, a, runMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeVSCodeDone {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeVSCodeDone)
	}
	if f.vscodeCalls != 1 {
		t.Errorf("OpenVSCode calls = %d, want 1", f.vscodeCalls)
	}
}

func TestStateVSCodeFailure(t *testing.T) {
	f := &stubFacade{vscodeErr: errors.New("VS Code not found")}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	a, _ = step(t, a, runMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeVSCodeErr+"VS Code not found" {
		t.Errorf("notice = %q", got)
	}
}

func TestStateBusyGateBlocksConcurrentActions(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, vscodeCmd := step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	if !a.busy {
		t.Fatal("busy = false after dispatching vscode, want true")
	}

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if got := menuNotice(t, a); got != uistr.NoticeBusy {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeBusy)
	}
	if f.launches() != 0 {
		t.Errorf("LaunchSteps called %d times while busy, want 0", f.launches())
	}

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	if got := menuNotice(t, a); got != uistr.NoticeBusy {
		t.Errorf("harness while busy: notice = %q, want %q", got, uistr.NoticeBusy)
	}

	a, _ = step(t, a, runMsg(t, vscodeCmd))
	if a.busy {
		t.Fatal("busy = true after vscode done, want false")
	}

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if f.launches() != 1 {
		t.Errorf("LaunchSteps calls after unbusy = %d, want 1", f.launches())
	}
	a = driveUntilDone(t, a)
	if a.pipe != nil {
		t.Error("pipe not nil after empty launch finished")
	}
}

func TestStateQuitCancelsContext(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, cmd := step(t, a, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if msg := runMsg(t, cmd); msg == nil {
		t.Error("q at menu depth produced nil msg, want quit")
	}
	if a.ctx.Err() == nil {
		t.Error("app ctx not cancelled after quit")
	}
}

func TestStateKeysTabEscAtMenu(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, cmd := step(t, a, tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd != nil {
		t.Errorf("tab at menu returned cmd %v, want nil", cmd)
	}
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyEscape})
	if a.ctx.Err() != nil {
		t.Error("esc at depth 1 cancelled ctx, want running")
	}
}

func TestStateLaunchFailureShowsFailedNotice(t *testing.T) {
	boom := &stateStep{
		meta: pipeline.Meta{Name: "boom", Title: "Boom", Kind: pipeline.Auto},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			return errors.New("exploded")
		},
	}
	dependent := &stateStep{
		meta: pipeline.Meta{Name: "after-boom", Title: "After", Deps: []string{"boom"}, Kind: pipeline.Auto},
	}
	f := &stubFacade{steps: []pipeline.Command{boom, dependent}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilDone(t, a)

	if a.pipe != nil {
		t.Error("pipe not nil after pipeline done")
	}
	if got := menuNotice(t, a); got != uistr.NoticeLaunchFailed {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeLaunchFailed)
	}
}

func TestStateLaunchEscShowsCanceledNotice(t *testing.T) {
	resumed := make(chan pipeline.Result, 1)
	f := &stubFacade{steps: []pipeline.Command{waitingStep("wait", struct{ X int }{1}, resumed)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilWaiting(t, a)
	if a.waiting != "wait" {
		t.Fatalf("waiting = %q, want wait", a.waiting)
	}

	a, _ = step(t, a, bus.ScreenPop{})
	select {
	case r := <-resumed:
		if !r.Cancelled {
			t.Error("Resume not Cancelled after ScreenPop")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline not resumed via ScreenPop")
	}

	a = driveUntilDone(t, a)
	if got := menuNotice(t, a); got != uistr.NoticeLaunchCanceled {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeLaunchCanceled)
	}
}

func TestStateScreenResultResumesWaiting(t *testing.T) {
	resumed := make(chan pipeline.Result, 1)
	f := &stubFacade{steps: []pipeline.Command{waitingStep("telegram", nil, resumed)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilWaiting(t, a)

	a, _ = step(t, a, bus.ScreenResult{Value: "tok"})
	select {
	case r := <-resumed:
		if r.Cancelled || r.Value != "tok" {
			t.Errorf("Resume = %+v, want Value=tok", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline not resumed via ScreenResult")
	}
	a = driveUntilDone(t, a)
	if got := menuNotice(t, a); got != "" {
		t.Errorf("notice = %q after clean finish, want empty", got)
	}
}

func TestStateAttachExecDonePipelineOwnsMenuReturn(t *testing.T) {
	oldRunner := execRunner
	var mu sync.Mutex
	var capturedCb tea.ExecCallback
	execRunner = func(_ tea.ExecCommand, fn tea.ExecCallback) tea.Cmd {
		mu.Lock()
		capturedCb = fn
		mu.Unlock()
		return nil
	}
	t.Cleanup(func() { execRunner = oldRunner })

	attach := &stateStep{
		meta: pipeline.Meta{Name: "attach", Title: "Attach", Kind: pipeline.Terminal},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: "attach", Argv: []string{"docker", "exec", "-it", "mirabilis", "claude"}}
			<-in
			return nil
		},
	}
	f := &stubFacade{steps: []pipeline.Command{attach}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilWaiting(t, a)

	mu.Lock()
	cb := capturedCb
	mu.Unlock()
	if cb == nil {
		t.Fatal("execRunner was not invoked for the attach argv")
	}

	a, _ = step(t, a, cb(nil))
	a = driveUntilDone(t, a)

	if a.pipe != nil {
		t.Error("pipe not nil after attach pipeline completed")
	}
	if a.router.Depth() != 1 {
		t.Errorf("router depth = %d after attach finished, want 1 (menu)", a.router.Depth())
	}
	if got := menuNotice(t, a); got != "" {
		t.Errorf("notice = %q after clean attach, want empty", got)
	}
}

func TestStateSecondLaunchWhilePipelineRuns(t *testing.T) {
	resumed := make(chan pipeline.Result, 1)
	f := &stubFacade{steps: []pipeline.Command{waitingStep("wait", struct{}{}, resumed)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilWaiting(t, a)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if f.launches() != 1 {
		t.Errorf("LaunchSteps calls = %d, want 1 (second launch gated)", f.launches())
	}

	a, _ = step(t, a, bus.ScreenPop{})
	<-resumed
	_ = driveUntilDone(t, a)
}

func TestStateLaunchPipelineNewError(t *testing.T) {
	dup1 := &stateStep{meta: pipeline.Meta{Name: "dup", Title: "Dup", Kind: pipeline.Auto}}
	dup2 := &stateStep{meta: pipeline.Meta{Name: "dup", Title: "Dup", Kind: pipeline.Auto}}
	f := &stubFacade{steps: []pipeline.Command{dup1, dup2}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if a.pipe != nil {
		t.Error("pipe set despite pipeline.New error")
	}
	notice := menuNotice(t, a)
	if notice == "" || notice == uistr.NoticeBusy {
		t.Errorf("notice = %q, want launch error", notice)
	}
}

func TestStateMenuChosenUnknownIsNoop(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)
	a, cmd := step(t, a, bus.MenuChosen{Action: "no-such-action"})
	if cmd != nil {
		t.Error("unknown action returned a cmd, want nil")
	}
	if a.busy || a.pipe != nil {
		t.Error("unknown action mutated state")
	}
}

func TestStateScreenPopClearsMenuAction(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	if a.menuAction != "reset" {
		t.Fatalf("menuAction = %q, want reset", a.menuAction)
	}
	a, _ = step(t, a, bus.ScreenPop{})
	if a.menuAction != "" {
		t.Errorf("menuAction = %q after pop, want empty", a.menuAction)
	}
	if f.saveCalls != 0 || f.resetCalls != 0 {
		t.Error("facade called on declined reset")
	}
}

func TestStateWatchStatusClosedChannel(t *testing.T) {
	ch := make(chan obs.Snapshot)
	close(ch)
	if msg := watchStatus(ch)(); msg != nil {
		t.Errorf("watchStatus on closed channel = %v, want nil", msg)
	}
}

func TestStatePipelineEventBranchesForward(t *testing.T) {
	ok := &stateStep{meta: pipeline.Meta{Name: "fine", Title: "Fine", Kind: pipeline.Auto},
		runFn: func(_ context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvSpawn, Step: "fine", Argv: []string{"docker", "version"}}
			out <- pipeline.Event{Kind: pipeline.EvLine, Step: "fine", Line: "stream line"}
			return nil
		}}
	boom := &stateStep{meta: pipeline.Meta{Name: "boom", Title: "Boom", Kind: pipeline.Auto},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			return errors.New("exploded")
		}}
	skipped := &stateStep{meta: pipeline.Meta{Name: "skipped", Title: "Skipped", Deps: []string{"boom"}, Kind: pipeline.Auto}}

	f := &stubFacade{steps: []pipeline.Command{ok, boom, skipped}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	seen := map[pipeline.EventKind]bool{}
	a = driveUntil(t, a, func(_ App, ev pipeline.Event) bool {
		seen[ev.Kind] = true
		return ev.Kind == pipeline.EvPipelineDone
	})

	for _, kind := range []pipeline.EventKind{pipeline.EvStepStarted, pipeline.EvSpawn, pipeline.EvLine, pipeline.EvDone, pipeline.EvFailed, pipeline.EvSkipped, pipeline.EvPipelineDone} {
		if !seen[kind] {
			t.Errorf("event kind %v never flowed through the bridge", kind)
		}
	}
	if got := menuNotice(t, a); got != uistr.NoticeLaunchFailed {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeLaunchFailed)
	}
}
