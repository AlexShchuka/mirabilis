package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/steps"
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
	resetErr         error
	lastHarness      string
	rememberedChoice string
	rememberErr      error
	telegramCfg      bool
	telegramMarked   bool
	markErr          error
	attachArgv       []string
	attachEnv        []string
	attachErr        error
	attachCalls      int
	statusSubs       int
}

func (f *stubFacade) LaunchSteps() []pipeline.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.launchCalls++
	return f.steps
}

func (f *stubFacade) Version() string { return "vtest" }

func (f *stubFacade) Logger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func (f *stubFacade) StatusUpdates() <-chan obs.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusSubs++
	return make(chan obs.Snapshot, 1)
}

func (f *stubFacade) statusUpdatesCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statusSubs
}

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
	return f.resetErr
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

func (f *stubFacade) AttachExec(context.Context) ([]string, []string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attachCalls++
	return f.attachArgv, f.attachEnv, f.attachErr
}

func (f *stubFacade) LastHarnessChoice() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastHarness
}

func (f *stubFacade) RememberHarnessChoice(choice string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rememberedChoice = choice
	return f.rememberErr
}

func (f *stubFacade) TelegramConfigured() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.telegramCfg
}

func (f *stubFacade) MarkTelegramConfigured() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.telegramMarked = true
	return f.markErr
}

func (f *stubFacade) remembered() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rememberedChoice
}

func (f *stubFacade) marked() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.telegramMarked
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
	a := New(context.Background(), f, false)
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

func drainBatch(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		if c == nil {
			continue
		}
		out = append(out, c())
	}
	return out
}

func runWorkMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	for _, m := range drainBatch(t, cmd) {
		if _, ok := m.(busyTickMsg); ok {
			continue
		}
		return m
	}
	t.Fatal("no work message produced (only busy ticks)")
	return nil
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

func wizardStep(name string, payload steps.Wizard, resumed chan pipeline.Result) *stateStep {
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

func collectFast(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	leaves := []tea.Cmd{cmd}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		leaves = batch
	}
	var out []tea.Msg
	for _, c := range leaves {
		if c == nil {
			continue
		}
		ch := make(chan tea.Msg, 1)
		go func(fn tea.Cmd) { ch <- fn() }(c)
		select {
		case msg := <-ch:
			if msg != nil {
				out = append(out, msg)
			}
		case <-time.After(200 * time.Millisecond):
		}
	}
	return out
}

func driveUntilFormUp(t *testing.T, a App) App {
	t.Helper()
	p := a.pipe
	if p == nil {
		t.Fatal("no pipeline running")
	}
	for {
		ev, ok := nextEvent(t, p)
		if !ok {
			t.Fatal("pipeline ended before a waiting form appeared")
		}
		var cmd tea.Cmd
		a, cmd = step(t, a, pipelineEventMsg{ev: ev})
		if ev.Kind != pipeline.EvWaiting {
			continue
		}
		for _, msg := range collectFast(t, cmd) {
			if _, ok := msg.(bus.ScreenPush); ok {
				a, _ = step(t, a, msg)
			}
		}
		return a
	}
}

func wizardOf(title string, options []string) steps.Wizard {
	return steps.Wizard{Groups: []steps.Catalog{
		{Key: "stacks", Title: title, Description: title, Options: options, MultiSelect: true},
	}}
}

func TestLaunchFormCompositesOverSteplist(t *testing.T) {
	resumed := make(chan pipeline.Result, 1)
	payload := wizardOf("choose-stacks", []string{"alpha", "beta"})
	f := &stubFacade{steps: []pipeline.Command{wizardStep("config", payload, resumed)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilFormUp(t, a)

	if a.router.Depth() != 3 {
		t.Fatalf("router depth = %d, want 3 (menu, launch, form)", a.router.Depth())
	}

	view := plainState(a.View().Content)
	if !strings.Contains(view, "config") {
		t.Errorf("composited view missing the launch steplist token (background):\n%s", view)
	}
	if !strings.Contains(view, "choose-stacks") {
		t.Errorf("composited view missing the wizard form token (overlay):\n%s", view)
	}
	if strings.Contains(view, uistr.WelcomeHint) {
		t.Errorf("background is the root menu, not the immediate parent launch screen:\n%s", view)
	}
}

func TestLaunchFormCompositesAndClampsAtSmallSize(t *testing.T) {
	resumed := make(chan pipeline.Result, 1)
	payload := wizardOf("choose-stacks", []string{"alpha", "beta"})
	f := &stubFacade{steps: []pipeline.Command{wizardStep("config", payload, resumed)}}
	a := newStateApp(t, f)

	const w, h = 50, 20
	a, _ = step(t, a, tea.WindowSizeMsg{Width: w, Height: h})
	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilFormUp(t, a)

	view := a.View().Content
	plain := plainState(view)
	if !strings.Contains(plain, "config") {
		t.Errorf("small composited view missing steplist token (background):\n%s", plain)
	}
	if !strings.Contains(plain, "choose-stacks") {
		t.Errorf("small composited view missing form token (overlay):\n%s", plain)
	}
	if got := lipgloss.Height(plain); got > h {
		t.Errorf("composited height = %d, want <= %d (Canvas must clamp)", got, h)
	}
	for i, line := range strings.Split(plain, "\n") {
		if got := lipgloss.Width(line); got > w {
			t.Errorf("composited line %d width = %d, want <= %d (Canvas must clamp)", i, got, w)
		}
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
	a, _ = step(t, a, runWorkMsg(t, cmd))
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
	a, _ = step(t, a, runWorkMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeTelegramErr+"boom: channel not detected" {
		t.Errorf("notice = %q", got)
	}
}

func TestStateResetConfirmRunsAndNotifies(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	if a.menuAction != "reset" {
		t.Fatalf("menuAction = %q, want reset", a.menuAction)
	}

	a, cmd := step(t, a, bus.ScreenResult{Value: true})
	if !a.busy {
		t.Error("busy = false while reset runs, want true")
	}
	a, _ = step(t, a, runWorkMsg(t, cmd))
	if a.busy {
		t.Error("busy = true after reset done, want false")
	}
	if got := menuNotice(t, a); got != uistr.NoticeResetDone {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeResetDone)
	}
	if f.saveCalls != 1 || f.resetCalls != 1 {
		t.Errorf("saveCalls=%d resetCalls=%d, want 1 and 1", f.saveCalls, f.resetCalls)
	}
}

func TestStateResetFailureNotice(t *testing.T) {
	f := &stubFacade{resetErr: errors.New("disk busy")}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	a, cmd := step(t, a, bus.ScreenResult{Value: true})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	if a.busy {
		t.Error("busy = true after failed reset, want false")
	}
	if got := menuNotice(t, a); got != uistr.NoticeResetFailed {
		t.Errorf("notice = %q, want %q", got, uistr.NoticeResetFailed)
	}
	if f.resetCalls != 1 {
		t.Errorf("resetCalls = %d, want 1", f.resetCalls)
	}
}

func TestStateResetCancelMakesNoCalls(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	a, _ = step(t, a, bus.ScreenPop{})
	if a.busy {
		t.Error("busy = true after reset cancel, want false")
	}
	if f.saveCalls != 0 || f.resetCalls != 0 {
		t.Errorf("saveCalls=%d resetCalls=%d after cancel, want 0 and 0", f.saveCalls, f.resetCalls)
	}
	if a.router.Depth() != 1 {
		t.Errorf("router depth = %d after pop, want 1 (menu only)", a.router.Depth())
	}
}

func TestStateHarnessFlow(t *testing.T) {
	f := &stubFacade{harnessCurrent: screens.HarnessOff}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	if !a.busy {
		t.Error("busy = false while harness status runs, want true")
	}
	a, _ = step(t, a, runWorkMsg(t, cmd))
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
	a, _ = step(t, a, runWorkMsg(t, cmd))
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
	a, _ = step(t, a, runWorkMsg(t, cmd))
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
	a, _ = step(t, a, runWorkMsg(t, cmd))
	a, cmd = step(t, a, bus.ScreenResult{Value: screens.HarnessOn})
	a, _ = step(t, a, runWorkMsg(t, cmd))
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
	a, _ = step(t, a, runWorkMsg(t, cmd))
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
	a, _ = step(t, a, runWorkMsg(t, cmd))
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

	a, _ = step(t, a, runWorkMsg(t, vscodeCmd))
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

func frameSelected(t *testing.T, a App) string {
	t.Helper()
	it, ok := a.frame.Selected()
	if !ok {
		t.Fatal("frame has no selected item")
	}
	return it.Action
}

func TestStateNavKeysDriveFrameCursorAtMenu(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	if got := frameSelected(t, a); got != screens.ActionLaunch {
		t.Fatalf("initial selection = %q, want launch", got)
	}

	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	if got := frameSelected(t, a); got != screens.ActionHarness {
		t.Errorf("after down: selection = %q, want harness", got)
	}

	a, _ = step(t, a, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if got := frameSelected(t, a); got != screens.ActionTelegram {
		t.Errorf("after j: selection = %q, want telegram", got)
	}

	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyUp})
	a, _ = step(t, a, tea.KeyPressMsg{Code: 'k', Text: "k"})
	if got := frameSelected(t, a); got != screens.ActionLaunch {
		t.Errorf("after up+k: selection = %q, want launch", got)
	}
}

func TestStateEnterAtMenuEmitsMenuChosen(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd := step(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := runMsg(t, cmd)
	chosen, ok := msg.(bus.MenuChosen)
	if !ok {
		t.Fatalf("enter at menu produced %T, want bus.MenuChosen", msg)
	}
	if chosen.Action != screens.ActionHarness {
		t.Errorf("enter emitted action %q, want harness", chosen.Action)
	}
}

func TestStateNavKeysIgnoredWhenOverlayUp(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	if a.router.Depth() != 2 {
		t.Fatalf("router depth = %d after reset push, want 2", a.router.Depth())
	}
	before := frameSelected(t, a)

	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	a, _ = step(t, a, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if got := frameSelected(t, a); got != before {
		t.Errorf("frame cursor moved while overlay up: %q -> %q", before, got)
	}
}

func plainState(s string) string {
	return regexp.MustCompile("\x1b\\[[0-9;<=>?]*[ -/]*[@-~]").ReplaceAllString(s, "")
}

func TestOverlayCompositesOverBackground(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})
	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	if a.router.Depth() != 2 {
		t.Fatalf("router depth = %d after reset push, want 2", a.router.Depth())
	}

	view := plainState(a.View().Content)
	if !strings.Contains(view, uistr.WelcomeHint) {
		t.Errorf("composited view lost the background welcome hint:\n%s", view)
	}
	if !strings.Contains(view, uistr.AppName) {
		t.Errorf("composited view lost the background frame header:\n%s", view)
	}
	if !strings.Contains(view, uistr.FormConfirmReset) {
		t.Errorf("composited view missing the overlay content:\n%s", view)
	}
}

func TestViewIsAltScreen(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})
	if !a.View().AltScreen {
		t.Error("View().AltScreen = false at menu depth, want true")
	}
	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	if !a.View().AltScreen {
		t.Error("View().AltScreen = false with overlay up, want true")
	}
}

func busyGlyphPresent(header string) bool {
	for _, g := range busyFrames {
		if strings.Contains(header, g) {
			return true
		}
	}
	return false
}

func TestBusyNoticeTicksElapsed(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})

	a, tickCmd := step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	if !a.busy {
		t.Fatal("busy = false after vscode dispatch, want true")
	}
	if tickCmd == nil {
		t.Fatal("vscode dispatch returned nil cmd, want batch with busy tick")
	}

	header := plainState(a.frame.View(""))
	if !busyGlyphPresent(strings.Split(header, "\n")[0]) {
		t.Errorf("frame header missing spinner glyph while busy:\n%s", strings.Split(header, "\n")[0])
	}

	a.busyStarted = a.busyStarted.Add(-3 * time.Second)
	a, c1 := step(t, a, busyTickMsg{at: time.Now(), gen: a.busyGen})
	if c1 == nil {
		t.Error("busy tick returned nil cmd while busy, want re-arm")
	}
	h1 := strings.Split(plainState(a.frame.View("")), "\n")[0]
	if !strings.Contains(h1, "3s") {
		t.Errorf("frame header missing elapsed seconds while busy:\n%s", h1)
	}
	if !busyGlyphPresent(h1) {
		t.Errorf("frame header missing spinner glyph after tick:\n%s", h1)
	}

	a, _ = step(t, a, runWorkMsg(t, tickCmd))
	if a.busy {
		t.Fatal("busy = true after vscode done, want false")
	}

	a, c2 := step(t, a, busyTickMsg{at: time.Now(), gen: a.busyGen})
	if c2 != nil {
		t.Error("busy tick re-armed after busy cleared, want self-terminating (nil cmd)")
	}
	hfinal := strings.Split(plainState(a.frame.View("")), "\n")[0]
	if busyGlyphPresent(hfinal) {
		t.Errorf("frame header still shows spinner after busy cleared:\n%s", hfinal)
	}
}

func TestBusyTickStaleGenIgnored(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})

	gen1 := a.busyGen
	a.busy = true
	cmd1 := a.startBusy()
	if cmd1 == nil {
		t.Fatal("first startBusy returned nil cmd, want a tick")
	}
	if a.busyGen != gen1+1 {
		t.Fatalf("busyGen = %d after first startBusy, want %d", a.busyGen, gen1+1)
	}

	cmd2 := a.startBusy()
	if cmd2 == nil {
		t.Fatal("second startBusy returned nil cmd, want a tick")
	}
	if a.busyGen != gen1+2 {
		t.Fatalf("busyGen = %d after second startBusy, want %d", a.busyGen, gen1+2)
	}

	frameBefore := a.busyFrame
	a, staleCmd := step(t, a, busyTickMsg{at: time.Now(), gen: gen1 + 1})
	if staleCmd != nil {
		t.Errorf("stale-gen tick re-armed: cmd = %v, want nil", staleCmd())
	}
	if a.busyFrame != frameBefore {
		t.Errorf("stale-gen tick advanced the frame: %d -> %d", frameBefore, a.busyFrame)
	}

	a, liveCmd := step(t, a, busyTickMsg{at: time.Now(), gen: a.busyGen})
	if liveCmd == nil {
		t.Error("live-gen tick returned nil cmd, want re-arm")
	}
	if a.busyFrame != frameBefore+1 {
		t.Errorf("live-gen tick did not advance the frame: %d -> %d", frameBefore, a.busyFrame)
	}
}

func TestHarnessRemembersChoiceOnSuccess(t *testing.T) {
	f := &stubFacade{harnessCurrent: screens.HarnessOff}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	a, cmd = step(t, a, bus.ScreenResult{Value: screens.HarnessReinstall})
	a, doneCmd := step(t, a, runWorkMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeHarnessDone {
		t.Fatalf("notice = %q, want harness done", got)
	}
	runMsg(t, doneCmd)
	if got := f.remembered(); got != screens.HarnessReinstall {
		t.Errorf("RememberHarnessChoice = %q, want reinstall", got)
	}
}

func TestHarnessDoesNotRememberOnFailure(t *testing.T) {
	f := &stubFacade{harnessCurrent: "missing", harnessApplyErr: errors.New("apply boom")}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	a, cmd = step(t, a, bus.ScreenResult{Value: screens.HarnessOn})
	_, _ = step(t, a, runWorkMsg(t, cmd))
	if got := f.remembered(); got != "" {
		t.Errorf("RememberHarnessChoice = %q after failure, want empty", got)
	}
}

func TestTelegramMarksConfiguredOnSuccess(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionTelegram})
	a, cmd := step(t, a, bus.ScreenResult{Value: "123:token"})
	_, doneCmd := step(t, a, runWorkMsg(t, cmd))
	runMsg(t, doneCmd)
	if !f.marked() {
		t.Error("MarkTelegramConfigured not called after successful configure")
	}
}

func TestTelegramDoesNotMarkOnFailure(t *testing.T) {
	f := &stubFacade{telegramErr: errors.New("configure boom")}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionTelegram})
	a, cmd := step(t, a, bus.ScreenResult{Value: "123:token"})
	_, _ = step(t, a, runWorkMsg(t, cmd))
	if f.marked() {
		t.Error("MarkTelegramConfigured called after failed configure")
	}
}

func TestHarnessStatusConsultsLastChoice(t *testing.T) {
	f := &stubFacade{harnessCurrent: screens.HarnessOff, lastHarness: screens.HarnessReinstall}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionHarness})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	if _, ok := a.router.Top().(screens.Harness); !ok {
		t.Fatalf("top screen = %T, want screens.Harness", a.router.Top())
	}
}

func TestMenuCursorSurvivesOverlay(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	before := frameSelected(t, a)
	if before != screens.ActionTelegram {
		t.Fatalf("setup: selection = %q, want telegram", before)
	}

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReset})
	if a.router.Depth() != 2 {
		t.Fatalf("router depth = %d after push, want 2", a.router.Depth())
	}
	a, _ = step(t, a, bus.ScreenPop{})
	if a.router.Depth() != 1 {
		t.Fatalf("router depth = %d after pop, want 1", a.router.Depth())
	}
	if got := frameSelected(t, a); got != before {
		t.Errorf("frame cursor changed across overlay round-trip: %q -> %q", before, got)
	}
}

func TestTypeaheadNotDroppedOnScreenTransition(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionTelegram})
	if _, ok := a.router.Top().(screens.Telegram); !ok {
		t.Fatalf("setup: top screen = %T, want screens.Telegram", a.router.Top())
	}
	depthBefore := a.router.Depth()

	for _, k := range []tea.KeyPressMsg{
		{Code: 'a', Text: "a"},
		{Code: 'b', Text: "b"},
		{Code: 'c', Text: "c"},
	} {
		a, _ = step(t, a, k)
	}

	if a.router.Depth() != depthBefore {
		t.Errorf("typed keys dropped the screen: depth %d -> %d", depthBefore, a.router.Depth())
	}
	if _, ok := a.router.Top().(screens.Telegram); !ok {
		t.Errorf("top screen = %T after typed keys, want telegram screen still present (typeahead buffered, not dropped)", a.router.Top())
	}
	if a.menuAction != "telegram" {
		t.Errorf("menuAction = %q after typeahead, want telegram (transition state preserved)", a.menuAction)
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

func TestStateGHAuthPushesScreenAndResumes(t *testing.T) {
	resumed := make(chan pipeline.Result, 1)
	payload := steps.GHAuth{Code: "ABCD-1234", URL: "https://github.com/login/device"}
	f := &stubFacade{steps: []pipeline.Command{waitingStep("gh-auth", payload, resumed)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilWaiting(t, a)

	if a.waiting != "gh-auth" {
		t.Fatalf("waiting = %q, want gh-auth", a.waiting)
	}
	if a.router.Depth() < 2 {
		t.Fatalf("router depth = %d, want >= 2 (gh-auth screen pushed)", a.router.Depth())
	}

	a, _ = step(t, a, bus.ScreenResult{Value: nil})
	select {
	case r := <-resumed:
		if r.Cancelled {
			t.Errorf("Resume cancelled, want completed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gh-auth not resumed via ScreenResult")
	}
}

func TestStateStatusSubscribedOnce(t *testing.T) {
	f := &stubFacade{}
	_ = newStateApp(t, f)
	if got := f.statusUpdatesCalls(); got != 1 {
		t.Errorf("StatusUpdates() called %d times at init, want exactly 1", got)
	}
}

func TestStateAttachExecDonePipelineOwnsMenuReturn(t *testing.T) {
	oldRunner := execRunner
	var mu sync.Mutex
	var capturedCmd tea.ExecCommand
	var capturedCb tea.ExecCallback
	execRunner = func(c tea.ExecCommand, fn tea.ExecCallback) tea.Cmd {
		mu.Lock()
		capturedCmd = c
		capturedCb = fn
		mu.Unlock()
		return nil
	}
	t.Cleanup(func() { execRunner = oldRunner })

	const fakeToken = "gho_test-secret-token"
	attach := &stateStep{
		meta: pipeline.Meta{Name: "attach", Title: "Attach", Kind: pipeline.Terminal},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind: pipeline.EvWaiting,
				Step: "attach",
				Argv: []string{"docker", "exec", "-it", "mirabilis", "claude"},
				Env:  []string{"GITHUB_PERSONAL_ACCESS_TOKEN=" + fakeToken},
			}
			<-in
			return nil
		},
	}
	f := &stubFacade{steps: []pipeline.Command{attach}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a = driveUntilWaiting(t, a)

	mu.Lock()
	cmd := capturedCmd
	cb := capturedCb
	mu.Unlock()
	if cb == nil {
		t.Fatal("execRunner was not invoked for the attach argv")
	}

	if tty, ok := cmd.(*exec.TTY); ok {
		for _, e := range tty.Env {
			if strings.Contains(e, fakeToken) {
				goto envOK
			}
		}
		t.Errorf("token not found in TTY.Env: %v", tty.Env)
		for _, a := range tty.Argv {
			if strings.Contains(a, fakeToken) {
				t.Errorf("token leaked into argv element: %q", a)
			}
		}
	envOK:
	} else {
		t.Errorf("captured exec command type = %T, want *exec.TTY", cmd)
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

func newSecondaryApp(t *testing.T, f Facade) App {
	t.Helper()
	a := New(context.Background(), f, true)
	t.Cleanup(a.cancel)
	return a
}

func frameEnabled(a App, action string) bool {
	return a.frame.Enabled(action)
}

func runningSnapshot() obs.Snapshot {
	return obs.Snapshot{"container": {State: obs.StateOK, Detail: "up"}}
}

func stoppedSnapshot() obs.Snapshot {
	return obs.Snapshot{"container": {State: obs.StateOff, Detail: "not running"}}
}

func TestStateSecondaryDisablesMutatingItems(t *testing.T) {
	f := &stubFacade{}
	a := newSecondaryApp(t, f)

	for _, action := range []string{screens.ActionLaunch, screens.ActionHarness, screens.ActionTelegram, screens.ActionReset} {
		if frameEnabled(a, action) {
			t.Errorf("secondary: action %q enabled, want disabled", action)
		}
	}
	for _, action := range []string{screens.ActionVSCode, screens.ActionQuit} {
		if !frameEnabled(a, action) {
			t.Errorf("secondary: action %q disabled, want enabled", action)
		}
	}
	if got := menuNotice(t, a); got != uistr.NoticeSecondary {
		t.Errorf("secondary notice = %q, want %q", got, uistr.NoticeSecondary)
	}
}

func TestStateSecondaryNavSkipsDisabled(t *testing.T) {
	f := &stubFacade{}
	a := newSecondaryApp(t, f)
	a, _ = step(t, a, statusMsg(runningSnapshot()))

	if got := frameSelected(t, a); got != screens.ActionVSCode {
		t.Fatalf("initial secondary selection = %q, want vscode (attach not yet first-enabled at build)", got)
	}
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyUp})
	if got := frameSelected(t, a); got != screens.ActionAttach {
		t.Errorf("after up: selection = %q, want attach (launch/harness/telegram disabled)", got)
	}
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	if got := frameSelected(t, a); got != screens.ActionQuit {
		t.Errorf("after down,down: selection = %q, want quit (reset disabled)", got)
	}
}

func TestStateSecondaryNoticeSurvivesBackToMenu(t *testing.T) {
	f := &stubFacade{vscodeErr: errors.New("boom")}
	a := newSecondaryApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeVSCodeErr+"boom" {
		t.Fatalf("notice = %q, want vscode err", got)
	}
}

func TestStatePromotionFlipsEnablementAndNotice(t *testing.T) {
	f := &stubFacade{}
	a := newSecondaryApp(t, f)

	a, _ = step(t, a, promotedMsg{})

	if a.secondary {
		t.Fatal("still secondary after promotion")
	}
	for _, action := range []string{screens.ActionLaunch, screens.ActionHarness, screens.ActionTelegram, screens.ActionReset} {
		if !frameEnabled(a, action) {
			t.Errorf("after promotion: action %q disabled, want enabled", action)
		}
	}
	if got := menuNotice(t, a); got != "" {
		t.Errorf("notice = %q after promotion, want empty", got)
	}
}

func TestStatePromotionNoopWhenOwner(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)
	a, _ = step(t, a, promotedMsg{})
	if a.secondary {
		t.Fatal("owner became secondary")
	}
}

func TestStateContainerStatusDrivesAttachEnablement(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	if frameEnabled(a, screens.ActionAttach) {
		t.Fatal("attach enabled before any container status")
	}

	a, _ = step(t, a, statusMsg(runningSnapshot()))
	if !frameEnabled(a, screens.ActionAttach) {
		t.Error("attach disabled while container running")
	}

	a, _ = step(t, a, statusMsg(stoppedSnapshot()))
	if frameEnabled(a, screens.ActionAttach) {
		t.Error("attach enabled while container stopped")
	}
}

func TestStateDegradedNodeStillLetsMenuNavigateAndDispatch(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})
	degraded := obs.Snapshot{
		"proxy":     {State: obs.StateDegraded, Detail: "token not ready"},
		"container": {State: obs.StateOK, Detail: "up"},
	}
	a, _ = step(t, a, statusMsg(degraded))

	view := plainState(a.View().Content)
	if !strings.Contains(view, uistr.DegradedPrefix+"proxy") {
		t.Errorf("status header does not surface the degraded proxy node:\n%s", view)
	}

	before := frameSelected(t, a)
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	moved := frameSelected(t, a)
	if moved == before {
		t.Errorf("nav broken while proxy degraded: cursor stuck at %q", before)
	}

	_, cmd := step(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := runMsg(t, cmd)
	chosen, ok := msg.(bus.MenuChosen)
	if !ok {
		t.Fatalf("enter produced %T while proxy degraded, want bus.MenuChosen", msg)
	}
	if chosen.Action != moved {
		t.Errorf("dispatch broken while degraded: action = %q, want %q (the selected item)", chosen.Action, moved)
	}

	if _, quit := step(t, a, tea.KeyPressMsg{Code: 'q', Text: "q"}); quit == nil {
		t.Error("q produced no command while proxy degraded, want quit")
	}
}

func TestStateAttachActionEmitsExecHandoff(t *testing.T) {
	oldRunner := execRunner
	var mu sync.Mutex
	var capturedCmd tea.ExecCommand
	var capturedCb tea.ExecCallback
	execRunner = func(c tea.ExecCommand, fn tea.ExecCallback) tea.Cmd {
		mu.Lock()
		capturedCmd = c
		capturedCb = fn
		mu.Unlock()
		return nil
	}
	t.Cleanup(func() { execRunner = oldRunner })

	const fakeToken = "gho_attach-secret"
	f := &stubFacade{
		attachArgv: []string{"docker", "exec", "-it", "mirabilis", "claude"},
		attachEnv:  []string{"GITHUB_PERSONAL_ACCESS_TOKEN=" + fakeToken},
	}
	a := newStateApp(t, f)
	a, _ = step(t, a, statusMsg(runningSnapshot()))

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionAttach})
	if got := menuNotice(t, a); got != uistr.NoticeAttachOpening {
		t.Fatalf("notice = %q, want %q", got, uistr.NoticeAttachOpening)
	}

	a, _ = step(t, a, runWorkMsg(t, cmd))
	if f.attachCalls != 1 {
		t.Fatalf("AttachExec calls = %d, want 1", f.attachCalls)
	}

	mu.Lock()
	gotCmd := capturedCmd
	cb := capturedCb
	mu.Unlock()
	if cb == nil {
		t.Fatal("execRunner not invoked for the attach action")
	}
	tty, ok := gotCmd.(*exec.TTY)
	if !ok {
		t.Fatalf("captured exec command = %T, want *exec.TTY", gotCmd)
	}
	for _, arg := range tty.Argv {
		if strings.Contains(arg, fakeToken) {
			t.Errorf("token leaked into argv element: %q", arg)
		}
	}
	foundEnv := false
	for _, e := range tty.Env {
		if e == "GITHUB_PERSONAL_ACCESS_TOKEN="+fakeToken {
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Errorf("token env missing from TTY.Env: %v", tty.Env)
	}

	a, _ = step(t, a, cb(nil))
	if a.busy {
		t.Error("busy after attach handoff returned")
	}
	if a.router.Depth() != 1 {
		t.Errorf("router depth = %d after attach, want 1 (menu)", a.router.Depth())
	}
}

func TestStateAttachActionNoTokenShowsError(t *testing.T) {
	f := &stubFacade{attachErr: errors.New("not logged in")}
	a := newStateApp(t, f)
	a, _ = step(t, a, statusMsg(runningSnapshot()))

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionAttach})
	a, _ = step(t, a, runWorkMsg(t, cmd))

	if got := menuNotice(t, a); got != uistr.NoticeAttachErr+"not logged in" {
		t.Errorf("notice = %q, want attach error", got)
	}
	if a.busy {
		t.Error("busy after attach error, want false")
	}
}
