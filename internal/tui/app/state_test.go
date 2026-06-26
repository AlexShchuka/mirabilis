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
	mu            sync.Mutex
	steps         []pipeline.Command
	loadouts      []LoadoutChoice
	selectedRole  string
	selectErr     error
	tuneEffort    string
	tuneFleet     bool
	tuneWriteErr  error
	tuneClearErr  error
	tuneWrites    []screens.TuneResult
	tuneClears    int
	launchCalls   int
	vscodeCalls   int
	vscodeErr     error
	updateCalls   int
	updateErr     error
	selfCalls     int
	selfErr       error
	statusSubs    int
	openURLCalls  []string
	willRecreate  bool
	recreateCalls int
	reviewMode    bool
	reviewSets    []bool
}

func (f *stubFacade) Loadouts() []LoadoutChoice {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loadouts
}

func (f *stubFacade) SelectLoadout(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.selectedRole = name
	return f.selectErr
}

func (f *stubFacade) selectedLoadout() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.selectedRole
}

func (f *stubFacade) EffectiveTune() (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tuneEffort, f.tuneFleet
}

func (f *stubFacade) WriteTune(effort string, fleet bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tuneWrites = append(f.tuneWrites, screens.TuneResult{Effort: effort, Fleet: fleet})
	return f.tuneWriteErr
}

func (f *stubFacade) ClearTune() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tuneClears++
	return f.tuneClearErr
}

func (f *stubFacade) tuneWriteCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tuneWrites)
}

func (f *stubFacade) lastTuneWrite() (screens.TuneResult, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.tuneWrites) == 0 {
		return screens.TuneResult{}, false
	}
	return f.tuneWrites[len(f.tuneWrites)-1], true
}

func (f *stubFacade) tuneClearCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tuneClears
}

func (f *stubFacade) WillRecreateContainer(context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recreateCalls++
	return f.willRecreate
}

func (f *stubFacade) recreateChecks() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recreateCalls
}

func (f *stubFacade) SetReviewMode(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reviewMode = on
	f.reviewSets = append(f.reviewSets, on)
}

func (f *stubFacade) reviewModeSets() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]bool, len(f.reviewSets))
	copy(out, f.reviewSets)
	return out
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

func (f *stubFacade) OpenVSCode(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vscodeCalls++
	return f.vscodeErr
}

func (f *stubFacade) UpdateEcosystem(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCalls++
	return f.updateErr
}

func (f *stubFacade) SelfUpdate(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.selfCalls++
	return f.selfErr
}

func (f *stubFacade) selfUpdates() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.selfCalls
}

func (f *stubFacade) updates() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateCalls
}

func (f *stubFacade) OpenURL(_ context.Context, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.openURLCalls = append(f.openURLCalls, url)
	return nil
}

func (f *stubFacade) openURLs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.openURLCalls))
	copy(out, f.openURLCalls)
	return out
}

func (f *stubFacade) CopyText(context.Context, string) error { return nil }

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

func launchSkipGate(t *testing.T, a App) App {
	t.Helper()
	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if _, ok := a.router.Top().(screens.UpdateGate); !ok {
		t.Fatalf("after ActionLaunch top screen is %T, want screens.UpdateGate", a.router.Top())
	}
	a, _ = step(t, a, bus.ScreenResult{Value: screens.GateSkip})
	return a
}

func pickRoleThenTune(t *testing.T, a App, role string) App {
	t.Helper()
	a, _ = step(t, a, bus.ScreenResult{Value: role})
	if !a.awaitingTune {
		t.Fatalf("awaitingTune = false after role %q, want true (tune step before launch)", role)
	}
	if _, ok := a.router.Top().(screens.Tune); !ok {
		t.Fatalf("top after role is %T, want screens.Tune", a.router.Top())
	}
	return a
}

func passTuneDefaults(t *testing.T, a App) App {
	t.Helper()
	a, _ = step(t, a, bus.ScreenPop{})
	if a.awaitingTune {
		t.Fatal("awaitingTune still true after esc, want false")
	}
	return a
}

func applyTune(t *testing.T, a App, res screens.TuneResult) App {
	t.Helper()
	a, _ = step(t, a, bus.ScreenResult{Value: res})
	if a.awaitingTune {
		t.Fatal("awaitingTune still true after apply, want false")
	}
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

func baseMenuNotice(t *testing.T, a App) string {
	t.Helper()
	if m, ok := a.router.Top().(screens.Menu); ok {
		return m.Notice()
	}
	if m, ok := a.router.Below().(screens.Menu); ok {
		return m.Notice()
	}
	t.Fatalf("neither top (%T) nor below is screens.Menu", a.router.Top())
	return ""
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

// collectDeep runs cmd, recursively unwraps tea.BatchMsg, and concurrently
// executes every leaf command (200 ms timeout each). Returns all resulting msgs.
func collectDeep(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	var leaves []tea.Cmd
	var expand func(tea.Cmd)
	expand = func(c tea.Cmd) {
		if c == nil {
			return
		}
		ch := make(chan tea.Msg, 1)
		go func() { ch <- c() }()
		select {
		case msg := <-ch:
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, bc := range batch {
					expand(bc)
				}
			} else if msg != nil {
				leaves = append(leaves, func() tea.Msg { return msg })
			}
		case <-time.After(200 * time.Millisecond):
		}
	}
	expand(cmd)
	var out []tea.Msg
	for _, c := range leaves {
		out = append(out, c())
	}
	return out
}

// driveUntilGHAuthUp drives the pipeline until EvWaiting with a GHAuth payload,
// then steps both bus.ScreenPush and openURLDoneMsg into the app so that the
// GHAuth screen is on the router stack and OpenURL has been called.
func driveUntilGHAuthUp(t *testing.T, a App) App {
	t.Helper()
	p := a.pipe
	if p == nil {
		t.Fatal("no pipeline running")
	}
	for {
		ev, ok := nextEvent(t, p)
		if !ok {
			t.Fatal("pipeline ended before gh-auth waiting appeared")
		}
		var cmd tea.Cmd
		a, cmd = step(t, a, pipelineEventMsg{ev: ev})
		if ev.Kind != pipeline.EvWaiting {
			continue
		}
		for _, msg := range collectDeep(t, cmd) {
			switch msg.(type) {
			case bus.ScreenPush, openURLDoneMsg:
				a, _ = step(t, a, msg)
			}
		}
		return a
	}
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

func TestLaunchScreenSizedToMainAreaNotFullTerminal(t *testing.T) {
	longLine := strings.Repeat("x", 200)
	streamer := &stateStep{meta: pipeline.Meta{Name: "stream", Title: "Stream", Kind: pipeline.Auto},
		runFn: func(_ context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvStepStarted, Step: "stream"}
			out <- pipeline.Event{Kind: pipeline.EvLine, Step: "stream", Line: longLine}
			<-make(chan struct{})
			return nil
		}}
	f := &stubFacade{steps: []pipeline.Command{streamer}}
	a := newStateApp(t, f)

	const w, h = 120, 40
	a, _ = step(t, a, tea.WindowSizeMsg{Width: w, Height: h})
	a = launchSkipGate(t, a)
	a = driveUntil(t, a, func(_ App, ev pipeline.Event) bool { return ev.Kind == pipeline.EvLine })

	mw, _ := a.frame.MainSize()
	if mw >= w {
		t.Fatalf("main-area width %d not narrower than terminal %d; menu chrome not subtracted", mw, w)
	}
	view := plainState(a.router.Top().View())
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > mw {
			t.Errorf("launch line %d width = %d, want <= main-area %d (screen got raw terminal size, not main area)", i, got, mw)
		}
	}
}

func TestLaunchFormCompositesOverSteplist(t *testing.T) {
	resumed := make(chan pipeline.Result, 1)
	payload := wizardOf("choose-stacks", []string{"alpha", "beta"})
	f := &stubFacade{steps: []pipeline.Command{wizardStep("config", payload, resumed)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	a = launchSkipGate(t, a)
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
	a = launchSkipGate(t, a)
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

func menuErrText(t *testing.T, a App) string {
	t.Helper()
	m, ok := a.router.Top().(screens.Menu)
	if !ok {
		t.Fatalf("top screen is %T, want screens.Menu", a.router.Top())
	}
	return m.ErrText()
}

func TestStateErrorSurfacePersistsAcrossBenignActionAndDismisses(t *testing.T) {
	f := &stubFacade{vscodeErr: errors.New("code missing")}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	a, _ = step(t, a, runWorkMsg(t, cmd))

	wantErr := uistr.NoticeVSCodeErr + "code missing"
	if got := menuErrText(t, a); got != wantErr {
		t.Fatalf("error surface = %q, want %q right after failure", got, wantErr)
	}

	f.mu.Lock()
	f.vscodeErr = nil
	f.mu.Unlock()
	a, cmd = step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	if got := menuNotice(t, a); got != uistr.NoticeVSCodeDone {
		t.Errorf("benign notice = %q, want %q", got, uistr.NoticeVSCodeDone)
	}
	if got := menuErrText(t, a); got != wantErr {
		t.Errorf("error surface = %q after a benign success, want it to persist as %q", got, wantErr)
	}

	a, _ = step(t, a, tea.KeyPressMsg{Code: 'x', Text: "x"})
	if got := menuErrText(t, a); got != "" {
		t.Errorf("error surface = %q after dismiss key, want empty", got)
	}
}

func TestStateErrorSurfaceReplacedByNextError(t *testing.T) {
	f := &stubFacade{vscodeErr: errors.New("first boom")}
	a := newStateApp(t, f)

	a, cmd := step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	wantFirst := uistr.NoticeVSCodeErr + "first boom"
	if got := menuErrText(t, a); got != wantFirst {
		t.Fatalf("first error = %q, want %q", got, wantFirst)
	}

	f.mu.Lock()
	f.vscodeErr = errors.New("second boom")
	f.mu.Unlock()
	a, cmd = step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	a, _ = step(t, a, runWorkMsg(t, cmd))
	want := uistr.NoticeVSCodeErr + "second boom"
	if got := menuErrText(t, a); got != want {
		t.Errorf("error surface = %q, want it replaced by the newer error %q", got, want)
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
	if a.awaitingGate {
		t.Error("update-gate pushed while busy, want gated")
	}
	if f.launches() != 0 {
		t.Errorf("LaunchSteps called %d times while busy, want 0", f.launches())
	}

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionVSCode})
	if got := menuNotice(t, a); got != uistr.NoticeBusy {
		t.Errorf("vscode while busy: notice = %q, want %q", got, uistr.NoticeBusy)
	}

	a, _ = step(t, a, runWorkMsg(t, vscodeCmd))
	if a.busy {
		t.Fatal("busy = true after vscode done, want false")
	}

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if !a.awaitingGate {
		t.Fatal("awaitingGate = false after launch when unbusy, want true")
	}
	a, _ = step(t, a, bus.ScreenResult{Value: screens.GateSkip})
	if f.launches() != 1 {
		t.Errorf("LaunchSteps calls after unbusy gate-skip = %d, want 1", f.launches())
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
	if got := frameSelected(t, a); got != screens.ActionReview {
		t.Errorf("after down: selection = %q, want review", got)
	}

	a, _ = step(t, a, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if got := frameSelected(t, a); got != screens.ActionVSCode {
		t.Errorf("after j: selection = %q, want vscode", got)
	}

	a, _ = step(t, a, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if got := frameSelected(t, a); got != screens.ActionQuit {
		t.Errorf("after second j: selection = %q, want quit", got)
	}

	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyUp})
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyUp})
	a, _ = step(t, a, tea.KeyPressMsg{Code: 'k', Text: "k"})
	if got := frameSelected(t, a); got != screens.ActionLaunch {
		t.Errorf("after up+up+k: selection = %q, want launch", got)
	}
}

func TestStateEnterAtMenuEmitsMenuChosen(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd := step(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := runMsg(t, cmd)
	chosen, ok := msg.(bus.MenuChosen)
	if !ok {
		t.Fatalf("enter at menu produced %T, want bus.MenuChosen", msg)
	}
	if chosen.Action != screens.ActionVSCode {
		t.Errorf("enter emitted action %q, want vscode", chosen.Action)
	}
}

func TestStateNavKeysIgnoredWhenOverlayUp(t *testing.T) {
	payload := wizardOf("choose-stacks", []string{"alpha", "beta"})
	f := &stubFacade{steps: []pipeline.Command{wizardStep("config", payload, nil)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	a = launchSkipGate(t, a)
	a = driveUntilFormUp(t, a)
	if a.router.Depth() != 3 {
		t.Fatalf("router depth = %d with form up, want 3", a.router.Depth())
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
	payload := wizardOf("choose-stacks", []string{"alpha", "beta"})
	f := &stubFacade{steps: []pipeline.Command{wizardStep("config", payload, nil)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})
	a = launchSkipGate(t, a)
	a = driveUntilFormUp(t, a)
	if a.router.Depth() != 3 {
		t.Fatalf("router depth = %d with form up, want 3", a.router.Depth())
	}

	v := a.View()
	view := plainState(v.Content)
	if !strings.Contains(view, "config") {
		t.Errorf("composited view lost the launch steplist background:\n%s", view)
	}
	if !strings.Contains(view, "choose-stacks") {
		t.Errorf("composited view missing the overlay content:\n%s", view)
	}
	if v.Cursor == nil {
		t.Error("overlay view has nil cursor; the form caret must show while an overlay is up")
	}
}

func TestViewIsAltScreen(t *testing.T) {
	payload := wizardOf("choose-stacks", []string{"alpha", "beta"})
	f := &stubFacade{steps: []pipeline.Command{wizardStep("config", payload, nil)}}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})
	if !a.View().AltScreen {
		t.Error("View().AltScreen = false at menu depth, want true")
	}
	if a.View().Cursor != nil {
		t.Error("menu view has a visible cursor; selection must be style-only (no caret)")
	}
	a = launchSkipGate(t, a)
	a = driveUntilFormUp(t, a)
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

func enableMotion(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("NO_ANIMATE", "")
	t.Setenv("ACCESSIBLE", "")
}

func TestBusyNoticeTicksElapsed(t *testing.T) {
	enableMotion(t)
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

func TestBusyStaticUnderReducedMotion(t *testing.T) {
	t.Setenv("NO_ANIMATE", "1")
	f := &stubFacade{}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})

	a.busy = true
	if cmd := a.startBusy(); cmd != nil {
		t.Error("startBusy returned a tick cmd under NO_ANIMATE, want nil (no animation; WCAG 2.3.3)")
	}
	h := strings.Split(plainState(a.frame.View("")), "\n")[0]
	if busyGlyphPresent(h) {
		t.Errorf("busy header shows an animated spinner frame under reduced motion:\n%s", h)
	}
	if !strings.Contains(h, busyStaticGlyph) {
		t.Errorf("busy header missing the static glyph under reduced motion:\n%s", h)
	}
}

func TestBusyTickStaleGenIgnored(t *testing.T) {
	enableMotion(t)
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

func TestMenuCursorSurvivesOverlay(t *testing.T) {
	payload := wizardOf("choose-stacks", []string{"alpha", "beta"})
	f := &stubFacade{steps: []pipeline.Command{wizardStep("config", payload, nil)}}
	a := newStateApp(t, f)

	a, _ = step(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyDown})
	before := frameSelected(t, a)
	if before != screens.ActionReview {
		t.Fatalf("setup: selection = %q, want review", before)
	}

	a = launchSkipGate(t, a)
	a = driveUntilFormUp(t, a)
	if a.router.Depth() != 3 {
		t.Fatalf("router depth = %d with form up, want 3", a.router.Depth())
	}
	a, _ = step(t, a, bus.ScreenPop{})
	a = driveUntilDone(t, a)
	if a.router.Depth() != 1 {
		t.Fatalf("router depth = %d after pop, want 1", a.router.Depth())
	}
	if got := frameSelected(t, a); got != before {
		t.Errorf("frame cursor changed across overlay round-trip: %q -> %q", before, got)
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

	a = launchSkipGate(t, a)
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

	a = launchSkipGate(t, a)
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

	a = launchSkipGate(t, a)
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

	a = launchSkipGate(t, a)
	// driveUntilGHAuthUp expands the nested batch and steps ScreenPush +
	// openURLDoneMsg into the app, placing GHAuth on the router stack.
	a = driveUntilGHAuthUp(t, a)

	if a.waiting != "gh-auth" {
		t.Fatalf("waiting = %q, want gh-auth", a.waiting)
	}
	if a.router.Depth() < 3 {
		t.Fatalf("router depth = %d, want >= 3 (menu, launch, ghauth)", a.router.Depth())
	}

	// OpenURL must have been fired automatically with the device URL.
	urls := f.openURLs()
	if len(urls) == 0 {
		t.Error("OpenURL not called after GHAuth screen push, want called with device URL")
	} else if urls[0] != payload.URL {
		t.Errorf("OpenURL called with %q, want %q", urls[0], payload.URL)
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

	a = launchSkipGate(t, a)
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

	a = launchSkipGate(t, a)
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

	a = launchSkipGate(t, a)
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

	a = launchSkipGate(t, a)
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

func frameEnabled(a App, action string) bool {
	return a.frame.Enabled(action)
}

func runningSnapshot() obs.Snapshot {
	return obs.Snapshot{"container": {State: obs.StateOK, Detail: "up"}}
}

func TestStateContainerStatusUpdatesFrame(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)
	a, _ = step(t, a, statusMsg(runningSnapshot()))
	if !frameEnabled(a, screens.ActionVSCode) {
		t.Error("vscode disabled after container running status")
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

func ghAuthApp(t *testing.T) (App, *stubFacade) {
	t.Helper()
	resumed := make(chan pipeline.Result, 1)
	payload := steps.GHAuth{Code: "ABCD-1234", URL: "https://github.com/login/device"}
	f := &stubFacade{steps: []pipeline.Command{waitingStep("gh-auth", payload, resumed)}}
	a := newStateApp(t, f)
	a = launchSkipGate(t, a)
	// driveUntilGHAuthUp expands the nested batch (ScreenPush + openURLDoneMsg)
	// and steps both into the app so GHAuth is on the router stack.
	a = driveUntilGHAuthUp(t, a)
	if a.waiting != "gh-auth" {
		t.Fatalf("ghAuthApp: waiting = %q, want gh-auth", a.waiting)
	}
	return a, f
}

func TestStateCopyDoneFeedbackRouted(t *testing.T) {
	a, _ := ghAuthApp(t)

	a, _ = step(t, a, copyDoneMsg{text: "ABCD-1234", err: nil})
	view := plainState(a.router.Top().View())
	if !strings.Contains(view, uistr.GHAuthCopied) {
		t.Errorf("copyDone(nil): ghauth view missing %q:\n%s", uistr.GHAuthCopied, view)
	}
}

func TestStateCopyDoneErrorFeedbackRouted(t *testing.T) {
	a, _ := ghAuthApp(t)

	a, _ = step(t, a, copyDoneMsg{text: "ABCD", err: errors.New("xclip not found")})
	view := plainState(a.router.Top().View())
	if !strings.Contains(view, uistr.GHAuthCopyFailed) {
		t.Errorf("copyDone(err): ghauth view missing %q:\n%s", uistr.GHAuthCopyFailed, view)
	}
}

func TestStateLaunchSetsBusyAndClearsOnDone(t *testing.T) {
	f := &stubFacade{steps: []pipeline.Command{&stateStep{
		meta: pipeline.Meta{Name: "noop", Title: "noop", Kind: pipeline.Auto},
	}}}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	if !a.busy {
		t.Fatal("busy = false after gate-skip starts the pipeline, want true")
	}

	a = driveUntilDone(t, a)
	if a.busy {
		t.Fatal("busy = true after pipelineDoneMsg, want false")
	}
}

func roleLoadouts() []LoadoutChoice {
	return []LoadoutChoice{
		{Key: "spark", Effort: "medium"},
		{Key: "drift", Effort: "high"},
		{Key: "orbit", Effort: "max"},
		{Key: "forge", Effort: "xhigh", Batch: true, Default: true},
		{Key: "nova", Effort: "max", Batch: true},
	}
}

func TestLaunchPresentsRolePickerBeforePipeline(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	if !a.awaitingRole {
		t.Fatal("awaitingRole = false after gate-skip, want true")
	}
	if a.pipe != nil {
		t.Fatal("pipe created before role chosen")
	}
	if f.launches() != 0 {
		t.Errorf("LaunchSteps called %d times before role chosen, want 0", f.launches())
	}
	if _, ok := a.router.Top().(screens.RolePicker); !ok {
		t.Fatalf("top screen is %T, want screens.RolePicker", a.router.Top())
	}
}

func TestRoleSelectionPersistsLoadoutThenLaunches(t *testing.T) {
	f := &stubFacade{
		loadouts: roleLoadouts(),
		steps:    []pipeline.Command{&stateStep{meta: pipeline.Meta{Name: "noop", Title: "noop", Kind: pipeline.Auto}}},
	}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a = passTuneDefaults(t, a)

	if a.awaitingRole {
		t.Error("awaitingRole still true after selection")
	}
	if got := f.selectedLoadout(); got != "orbit" {
		t.Errorf("SelectLoadout got %q, want orbit", got)
	}
	if f.launches() != 1 {
		t.Errorf("LaunchSteps calls = %d, want 1", f.launches())
	}
	if a.pipe == nil {
		t.Fatal("pipe nil after role chosen, want running pipeline")
	}
	a = driveUntilDone(t, a)
}

func TestRoleSelectionEnterDrivesFullFlow(t *testing.T) {
	f := &stubFacade{
		loadouts: roleLoadouts(),
		steps:    []pipeline.Command{&stateStep{meta: pipeline.Meta{Name: "noop", Title: "noop", Kind: pipeline.Auto}}},
	}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 120, Height: 40})

	a = launchSkipGate(t, a)
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyLeft})
	_, cmd := step(t, a, tea.KeyPressMsg{Code: tea.KeyEnter})
	res := runMsg(t, cmd)
	sr, ok := res.(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter on role picker produced %T, want bus.ScreenResult", res)
	}
	a, _ = step(t, a, sr)
	if !a.awaitingTune {
		t.Fatal("awaitingTune = false after enter-selecting role, want true (tune step)")
	}
	a = passTuneDefaults(t, a)

	if got := f.selectedLoadout(); got != "orbit" {
		t.Errorf("SelectLoadout got %q, want orbit", got)
	}
	if a.pipe == nil {
		t.Fatal("pipe nil after enter-selecting role")
	}
	a = driveUntilDone(t, a)
}

func TestRoleSelectionCancelledReturnsToMenu(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: []pipeline.Command{&stateStep{
		meta: pipeline.Meta{Name: "noop", Title: "noop", Kind: pipeline.Auto},
	}}}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a, _ = step(t, a, bus.ScreenPop{})

	if a.awaitingRole {
		t.Error("awaitingRole still true after cancel")
	}
	if a.pipe != nil {
		t.Error("pipe created after cancel, want none")
	}
	if f.launches() != 0 {
		t.Errorf("LaunchSteps called %d times after cancel, want 0", f.launches())
	}
	if _, ok := a.router.Top().(screens.Menu); !ok {
		t.Fatalf("top screen after cancel is %T, want screens.Menu", a.router.Top())
	}
}

func TestRoleSelectionEscKeyCancelsToMenu(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 120, Height: 40})

	a = launchSkipGate(t, a)
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyEscape})

	if a.awaitingRole {
		t.Error("awaitingRole still true after esc")
	}
	if a.pipe != nil {
		t.Error("pipe created after esc cancel")
	}
	if _, ok := a.router.Top().(screens.Menu); !ok {
		t.Fatalf("top screen after esc is %T, want screens.Menu", a.router.Top())
	}
}

func TestLaunchSkipsRolePickerWhenNoLoadouts(t *testing.T) {
	f := &stubFacade{steps: []pipeline.Command{&stateStep{
		meta: pipeline.Meta{Name: "noop", Title: "noop", Kind: pipeline.Auto},
	}}}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	if a.awaitingRole {
		t.Error("awaitingRole = true with empty loadout catalog, want false")
	}
	if a.pipe == nil {
		t.Fatal("pipe nil with empty catalog, want direct launch")
	}
	if f.selectedLoadout() != "" {
		t.Errorf("SelectLoadout called with empty catalog: %q", f.selectedLoadout())
	}
	a = driveUntilDone(t, a)
}

func noopSteps() []pipeline.Command {
	return []pipeline.Command{&stateStep{meta: pipeline.Meta{Name: "noop", Title: "noop", Kind: pipeline.Auto}}}
}

func openGate(t *testing.T, a App, choice string) (App, tea.Cmd) {
	t.Helper()
	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if !a.awaitingGate {
		t.Fatal("awaitingGate = false after ActionLaunch, want true")
	}
	if _, ok := a.router.Top().(screens.UpdateGate); !ok {
		t.Fatalf("top after launch is %T, want screens.UpdateGate", a.router.Top())
	}
	return step(t, a, bus.ScreenResult{Value: choice})
}

func TestLaunchPushesUpdateGateBeforeParty(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	if !a.awaitingGate {
		t.Fatal("awaitingGate = false after ActionLaunch, want true")
	}
	if a.awaitingRole {
		t.Error("awaitingRole = true before the gate is resolved")
	}
	if f.selfUpdates() != 0 || f.updates() != 0 {
		t.Errorf("update ran before gate choice: self=%d packs=%d", f.selfUpdates(), f.updates())
	}
	if _, ok := a.router.Top().(screens.UpdateGate); !ok {
		t.Fatalf("top screen is %T, want screens.UpdateGate", a.router.Top())
	}
}

func TestGateCarriesVersionContextFromSnapshot(t *testing.T) {
	f := &stubFacade{}
	a := newStateApp(t, f)

	a, _ = step(t, a, statusMsg(obs.Snapshot{
		uistr.VersionNode: {State: obs.StateDegraded, Detail: "v9.9.9"},
	}))
	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})

	gate, ok := a.router.Top().(screens.UpdateGate)
	if !ok {
		t.Fatalf("top is %T, want screens.UpdateGate", a.router.Top())
	}
	view := plainState(gate.View())
	if !strings.Contains(view, "v9.9.9") {
		t.Errorf("gate view missing latest tag from snapshot:\n%s", view)
	}
	if !strings.Contains(view, f.Version()) {
		t.Errorf("gate view missing current version %q:\n%s", f.Version(), view)
	}
}

func TestGateSkipGoesStraightToParty(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)

	a, _ = openGate(t, a, screens.GateSkip)
	if f.selfUpdates() != 0 || f.updates() != 0 {
		t.Errorf("skip ran an update: self=%d packs=%d", f.selfUpdates(), f.updates())
	}
	if !a.awaitingRole {
		t.Fatal("awaitingRole = false after gate skip, want true (party pick)")
	}
	if _, ok := a.router.Top().(screens.RolePicker); !ok {
		t.Fatalf("top after skip is %T, want screens.RolePicker", a.router.Top())
	}
}

func TestGatePacksRunsEcosystemThenParty(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)

	a, cmd := openGate(t, a, screens.GatePacks)
	if !a.busy {
		t.Fatal("busy = false while packs update runs, want true")
	}
	a, _ = step(t, a, runWorkMsg(t, cmd))

	if f.updates() != 1 {
		t.Errorf("UpdateEcosystem calls = %d, want 1", f.updates())
	}
	if f.selfUpdates() != 0 {
		t.Errorf("SelfUpdate calls = %d, want 0 for packs", f.selfUpdates())
	}
	if !a.awaitingRole {
		t.Fatal("awaitingRole = false after packs done, want true (party pick)")
	}
}

func TestGateSelfStagesNoticeThenParty(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)

	a, cmd := openGate(t, a, screens.GateSelf)
	if got := menuNotice(t, a); got != uistr.NoticeSelfUpdateRunning {
		t.Errorf("running notice = %q, want %q", got, uistr.NoticeSelfUpdateRunning)
	}
	a, _ = step(t, a, runWorkMsg(t, cmd))

	if f.selfUpdates() != 1 {
		t.Errorf("SelfUpdate calls = %d, want 1", f.selfUpdates())
	}
	if f.updates() != 0 {
		t.Errorf("UpdateEcosystem calls = %d, want 0 for self", f.updates())
	}
	if !a.awaitingRole {
		t.Fatal("awaitingRole = false after self staged, want true (party pick)")
	}
	if got := baseMenuNotice(t, a); got != uistr.NoticeSelfUpdateStaged {
		t.Errorf("staged notice = %q, want %q", got, uistr.NoticeSelfUpdateStaged)
	}
}

func TestGateSelfFailureDegradesButProceeds(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), selfErr: errors.New("not a fast-forward")}
	a := newStateApp(t, f)

	a, cmd := openGate(t, a, screens.GateSelf)
	a, _ = step(t, a, runWorkMsg(t, cmd))

	if !a.awaitingRole {
		t.Fatal("self-update failure blocked the launch; awaitingRole = false, want true (G6 degrade not block)")
	}
	notice := baseMenuNotice(t, a)
	if !strings.HasPrefix(notice, uistr.NoticeSelfUpdateDegraded) {
		t.Errorf("degraded notice = %q, want prefix %q", notice, uistr.NoticeSelfUpdateDegraded)
	}
	if !strings.Contains(notice, "not a fast-forward") {
		t.Errorf("degraded notice = %q, want the underlying error surfaced", notice)
	}
}

func TestGateAllRunsSelfThenPacksThenParty(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)

	a, cmd := openGate(t, a, screens.GateAll)
	a, cmd = step(t, a, runWorkMsg(t, cmd))
	if f.selfUpdates() != 1 {
		t.Fatalf("after self phase: SelfUpdate calls = %d, want 1", f.selfUpdates())
	}
	if !a.busy {
		t.Fatal("busy = false between self and packs phases, want true")
	}
	a, _ = step(t, a, runWorkMsg(t, cmd))

	if f.updates() != 1 {
		t.Errorf("UpdateEcosystem calls = %d, want 1 (packs phase of All)", f.updates())
	}
	if !a.awaitingRole {
		t.Fatal("awaitingRole = false after All done, want true (party pick)")
	}
}

func TestGateEscSkipsToParty(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts()}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyEscape})

	if f.selfUpdates() != 0 || f.updates() != 0 {
		t.Errorf("esc ran an update: self=%d packs=%d", f.selfUpdates(), f.updates())
	}
	if !a.awaitingRole {
		t.Fatal("awaitingRole = false after esc on gate, want true (esc skips to party)")
	}
}

func TestGatePacksWithNoLoadoutsLaunchesPipeline(t *testing.T) {
	f := &stubFacade{steps: noopSteps()}
	a := newStateApp(t, f)

	a, cmd := openGate(t, a, screens.GatePacks)
	a, _ = step(t, a, runWorkMsg(t, cmd))

	if f.updates() != 1 {
		t.Errorf("UpdateEcosystem calls = %d, want 1", f.updates())
	}
	if a.pipe == nil {
		t.Fatal("pipe nil after packs with empty catalog, want direct launch")
	}
	a = driveUntilDone(t, a)
}

func TestRestartWarnShownBeforeDestructiveRecreateAfterParty(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps(), willRecreate: true}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	if !a.awaitingRole {
		t.Fatal("awaitingRole = false after gate skip, want true (party pick)")
	}

	a = pickRoleThenTune(t, a, "orbit")
	a = passTuneDefaults(t, a)
	if a.pipe != nil {
		t.Fatal("pipe created before the restart warning was answered, want destructive recreate gated")
	}
	if f.launches() != 0 {
		t.Errorf("LaunchSteps called %d times before restart confirmed, want 0", f.launches())
	}
	if !a.awaitingRestart {
		t.Fatal("awaitingRestart = false after a recreate-bound launch, want true")
	}
	if _, ok := a.router.Top().(screens.RestartWarn); !ok {
		t.Fatalf("top screen is %T, want screens.RestartWarn", a.router.Top())
	}
}

func TestRestartWarnConfirmProceedsToLaunch(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps(), willRecreate: true}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a = passTuneDefaults(t, a)
	if _, ok := a.router.Top().(screens.RestartWarn); !ok {
		t.Fatalf("setup: top is %T, want screens.RestartWarn", a.router.Top())
	}

	a, _ = step(t, a, bus.ScreenResult{Value: screens.RestartConfirm})
	if a.awaitingRestart {
		t.Error("awaitingRestart still true after confirm")
	}
	if a.pipe == nil {
		t.Fatal("pipe nil after restart confirmed, want launch running")
	}
	if f.launches() != 1 {
		t.Errorf("LaunchSteps calls after confirm = %d, want 1", f.launches())
	}
	a = driveUntilDone(t, a)
}

func TestRestartWarnCancelAbortsToMenu(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps(), willRecreate: true}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a = passTuneDefaults(t, a)
	a, _ = step(t, a, bus.ScreenResult{Value: screens.RestartCancel})

	if a.awaitingRestart {
		t.Error("awaitingRestart still true after cancel")
	}
	if a.pipe != nil {
		t.Error("pipe created after restart cancelled, want none (no recreate)")
	}
	if f.launches() != 0 {
		t.Errorf("LaunchSteps called %d times after cancel, want 0 (clean abort, no recreate)", f.launches())
	}
	if _, ok := a.router.Top().(screens.Menu); !ok {
		t.Fatalf("top screen after cancel is %T, want screens.Menu", a.router.Top())
	}
}

func TestRestartWarnEscAbortsToMenu(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps(), willRecreate: true}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 120, Height: 40})

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a = passTuneDefaults(t, a)
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyEscape})

	if a.awaitingRestart {
		t.Error("awaitingRestart still true after esc")
	}
	if a.pipe != nil {
		t.Error("pipe created after esc on restart warning, want none")
	}
	if _, ok := a.router.Top().(screens.Menu); !ok {
		t.Fatalf("top screen after esc is %T, want screens.Menu", a.router.Top())
	}
}

func TestHealthyRelaunchSkipsRestartWarning(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps(), willRecreate: false}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a = passTuneDefaults(t, a)

	if a.awaitingRestart {
		t.Fatal("awaitingRestart = true on a healthy relaunch, want false (I9 zero questions)")
	}
	if a.pipe == nil {
		t.Fatal("pipe nil on a healthy relaunch, want direct launch (no warning)")
	}
	if f.recreateChecks() == 0 {
		t.Error("WillRecreateContainer never consulted before launch")
	}
	a = driveUntilDone(t, a)
}

func TestNoLoadoutsDestructiveRecreateStillWarns(t *testing.T) {
	f := &stubFacade{steps: noopSteps(), willRecreate: true}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	if a.pipe != nil {
		t.Fatal("pipe created without the restart warning on the no-loadout path")
	}
	if !a.awaitingRestart {
		t.Fatal("awaitingRestart = false on the no-loadout destructive path, want true")
	}
	if _, ok := a.router.Top().(screens.RestartWarn); !ok {
		t.Fatalf("top screen is %T, want screens.RestartWarn", a.router.Top())
	}
	a, _ = step(t, a, bus.ScreenResult{Value: screens.RestartConfirm})
	if a.pipe == nil {
		t.Fatal("pipe nil after confirming no-loadout restart, want launch")
	}
	a = driveUntilDone(t, a)
}

func TestTuneShownAfterPartyPick(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), tuneEffort: "high", tuneFleet: true}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 120, Height: 40})

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")

	if got := f.selectedLoadout(); got != "orbit" {
		t.Errorf("SelectLoadout got %q, want orbit (party persisted before tune)", got)
	}
	if a.pipe != nil {
		t.Fatal("pipe created before tune answered, want launch gated behind tune")
	}
	tune, ok := a.router.Top().(screens.Tune)
	if !ok {
		t.Fatalf("top is %T, want screens.Tune", a.router.Top())
	}
	view := plainState(tune.View())
	if !strings.Contains(view, "high") {
		t.Errorf("tune view missing pre-filled effort %q:\n%s", "high", view)
	}
	if !strings.Contains(view, uistr.TuneFleetOn) {
		t.Errorf("tune view missing pre-filled fleet on:\n%s", view)
	}
}

func TestTuneApplyWritesOverrideThenLaunches(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps(), tuneEffort: "medium"}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a = applyTune(t, a, screens.TuneResult{Effort: "max", Fleet: true})

	if f.tuneWriteCount() != 1 {
		t.Fatalf("WriteTune calls = %d, want 1", f.tuneWriteCount())
	}
	got, _ := f.lastTuneWrite()
	if got.Effort != "max" || !got.Fleet {
		t.Errorf("WriteTune got %+v, want {max true}", got)
	}
	if f.tuneClearCount() != 0 {
		t.Errorf("ClearTune called %d times on apply, want 0", f.tuneClearCount())
	}
	if a.pipe == nil {
		t.Fatal("pipe nil after applying tune, want launch running")
	}
	a = driveUntilDone(t, a)
}

func TestTuneEscClearsOverrideThenLaunches(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps()}
	a := newStateApp(t, f)
	a, _ = step(t, a, tea.WindowSizeMsg{Width: 120, Height: 40})

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a, _ = step(t, a, tea.KeyPressMsg{Code: tea.KeyEscape})

	if a.awaitingTune {
		t.Error("awaitingTune still true after esc")
	}
	if f.tuneClearCount() != 1 {
		t.Fatalf("ClearTune calls = %d, want 1 (esc keeps party defaults)", f.tuneClearCount())
	}
	if f.tuneWriteCount() != 0 {
		t.Errorf("WriteTune called %d times on esc, want 0", f.tuneWriteCount())
	}
	if a.pipe == nil {
		t.Fatal("pipe nil after esc on tune, want launch with party defaults")
	}
	a = driveUntilDone(t, a)
}

func TestTuneEscWithDestructiveRecreateStillWarns(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), steps: noopSteps(), willRecreate: true}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a = pickRoleThenTune(t, a, "orbit")
	a = passTuneDefaults(t, a)

	if a.pipe != nil {
		t.Fatal("pipe created on esc-tune before restart warning, want recreate gated")
	}
	if !a.awaitingRestart {
		t.Fatal("awaitingRestart = false after esc-tune on destructive recreate, want true")
	}
}

func TestLaunchWithNoLoadoutsSkipsTune(t *testing.T) {
	f := &stubFacade{steps: noopSteps()}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)

	if a.awaitingTune {
		t.Error("awaitingTune = true with empty loadout catalog, want false (no party, no tune)")
	}
	if f.tuneWriteCount() != 0 || f.tuneClearCount() != 0 {
		t.Errorf("tune store touched on the no-loadout path: writes=%d clears=%d", f.tuneWriteCount(), f.tuneClearCount())
	}
	if a.pipe == nil {
		t.Fatal("pipe nil with empty catalog, want direct launch")
	}
	a = driveUntilDone(t, a)
}

func TestTuneRoleSelectErrorReturnsToMenu(t *testing.T) {
	f := &stubFacade{loadouts: roleLoadouts(), selectErr: errors.New("disk full")}
	a := newStateApp(t, f)

	a = launchSkipGate(t, a)
	a, _ = step(t, a, bus.ScreenResult{Value: "orbit"})

	if a.awaitingTune {
		t.Error("awaitingTune = true after SelectLoadout error, want false")
	}
	if f.tuneWriteCount() != 0 {
		t.Errorf("WriteTune called after select error: %d", f.tuneWriteCount())
	}
	if a.pipe != nil {
		t.Error("pipe created after select error, want none")
	}
}

func TestStateReviewActionSetsReviewModeBeforeLaunch(t *testing.T) {
	reviewStep := &stateStep{meta: pipeline.Meta{Name: "review-present", Title: "Review", Kind: pipeline.Terminal}}
	f := &stubFacade{steps: []pipeline.Command{reviewStep}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionReview})

	if got := f.reviewModeSets(); len(got) != 1 || !got[0] {
		t.Fatalf("ActionReview SetReviewMode calls = %v, want exactly [true]", got)
	}
	if f.launches() != 1 {
		t.Errorf("ActionReview LaunchSteps calls = %d, want 1", f.launches())
	}
	a = driveUntilDone(t, a)
	if a.pipe != nil {
		t.Error("pipe not nil after review run finished")
	}
}

func TestStateNormalLaunchDoesNotSetReviewMode(t *testing.T) {
	launchStep := &stateStep{meta: pipeline.Meta{Name: "attach", Title: "Attach", Kind: pipeline.Terminal}}
	f := &stubFacade{steps: []pipeline.Command{launchStep}}
	a := newStateApp(t, f)

	a, _ = step(t, a, bus.MenuChosen{Action: screens.ActionLaunch})
	a, _ = step(t, a, bus.ScreenResult{Value: screens.GateSkip})

	if f.launches() != 1 {
		t.Fatalf("normal launch LaunchSteps calls = %d, want 1", f.launches())
	}
	for _, on := range f.reviewModeSets() {
		if on {
			t.Errorf("normal launch set review mode on: sets = %v, want no true", f.reviewModeSets())
			break
		}
	}
	a = driveUntilDone(t, a)
	if a.pipe != nil {
		t.Error("pipe not nil after normal launch finished")
	}
}
