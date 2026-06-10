package app

import (
	"context"
	"errors"
	"io"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/AlexShchuka/mirabilis/internal/ghauth"
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestAppCtrlC_CancelsLiveGHAuth(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	if waiting.ghCancel == nil {
		t.Fatal("precondition: ghCancel must be set in phaseGHAuth")
	}
	canceled := false
	waiting.ghCancel = func() { canceled = true }

	a, cmd := waiting.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !canceled {
		t.Error("ctrl+c with a live gh context must cancel it")
	}
	if asApp(t, a).ghCancel != nil {
		t.Error("ghCancel must be cleared after ctrl+c")
	}
	if cmd == nil {
		t.Fatal("ctrl+c must still quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c emitted %T, want tea.Quit", cmd())
	}
}

func TestAppBackToMenuMsg_ReturnsToMenu(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, backToMenuMsg{notice: "done"}))
	if a.phase != phaseMenu {
		t.Errorf("phase = %v, want menu after backToMenuMsg", a.phase)
	}
	if a.notice != "done" {
		t.Errorf("notice = %q, want carried through to the menu", a.notice)
	}
}

func TestAppGHDone_NoPipe_ReturnsToMenu(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	waiting.pipe = nil

	a := asApp(t, mustUpdate(waiting, ghauth.DoneMsg{Err: nil}))
	if a.phase != phaseMenu {
		t.Errorf("phase = %v, want menu when gh finishes with no pipeline to resume", a.phase)
	}
}

func TestAppCheckedMsg_ForwardsToPipe(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	if launched.pipe == nil {
		t.Fatal("precondition: pipe must be live")
	}
	a := asApp(t, mustUpdate(launched, pipeline.CheckedMsg{Name: "nope", Satisfied: true}))
	if a.phase != phasePipeline {
		t.Errorf("phase = %v, want pipeline — a CheckedMsg must stay in the pipeline", a.phase)
	}
}

func TestAppPipelineEnd_NonKeyMsg_Ignored(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	failed := asApp(t, mustUpdate(launched, pipeline.DoneMsg{Failed: true}))
	if !failed.pipeEnd {
		t.Fatal("precondition: pipeEnd must be true")
	}
	a, cmd := failed.Update(unhandledMsg{})
	if asApp(t, a).phase != phasePipeline {
		t.Errorf("phase = %v, want pipeline — a non-key msg after pipeEnd must be ignored", asApp(t, a).phase)
	}
	if cmd != nil {
		t.Error("a non-key msg after pipeEnd should produce no cmd")
	}
}

type unhandledMsg struct{}

func TestAppPhasePipeline_ForwardsOrdinaryMsg(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, tea.KeyPressMsg{Code: 'j', Text: "j"}))
	if a.phase != phasePipeline {
		t.Errorf("phase = %v, want pipeline — an ordinary key must be forwarded, not exit", a.phase)
	}
}

func TestAppPhaseGHAuth_ForwardsOrdinaryMsg(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	if waiting.gh == nil {
		t.Fatal("precondition: gh child must be set")
	}
	a := asApp(t, mustUpdate(waiting, tea.KeyPressMsg{Code: 'k', Text: "k"}))
	if a.phase != phaseGHAuth {
		t.Errorf("phase = %v, want ghauth — an ordinary key must be forwarded to the gh child", a.phase)
	}
}

func TestAppUpdateForm_CompletedApplies(t *testing.T) {
	a := newTestApp()
	f := newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	f.form.State = huh.StateCompleted
	a.form = f
	a.phase = phaseForm

	_, cmd := a.updateForm(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if cmd == nil {
		t.Fatal("updateForm on a completed form must run apply()")
	}
	if _, ok := cmd().(backToMenuMsg); !ok {
		t.Errorf("completed reset form apply emitted %T, want backToMenuMsg", cmd())
	}
}

func TestAppUpdate_PhaseForm_DispatchesToUpdateForm(t *testing.T) {
	a := newTestApp()
	a.form = newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	a.phase = phaseForm
	result := asApp(t, mustUpdate(a, tea.KeyPressMsg{Code: 'j', Text: "j"}))
	if result.phase != phaseForm {
		t.Errorf("phase = %v, want form — an ordinary key in phaseForm routes through updateForm", result.phase)
	}
}

func TestAppUpdate_UnknownPhase_ReturnsModelUnchanged(t *testing.T) {
	a := newTestApp()
	a.phase = appPhase(99)
	result, cmd := a.Update(unhandledMsg{})
	if asApp(t, result).phase != appPhase(99) {
		t.Error("an unknown phase must leave the model unchanged")
	}
	if cmd != nil {
		t.Error("an unknown phase must produce no cmd")
	}
}

func TestAppUpdateForm_InProgress_ForwardsCmd(t *testing.T) {
	a := newTestApp()
	a.form = newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	a.phase = phaseForm
	if a.form.completed() || a.form.aborted() {
		t.Fatal("precondition: a fresh form is neither completed nor aborted")
	}
	a2, _ := a.updateForm(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if asApp(t, a2).phase != phaseForm {
		t.Errorf("phase = %v, want form — an in-progress form keeps the form phase", asApp(t, a2).phase)
	}
}

func TestAppToMenu_CancelsLiveGHContext(t *testing.T) {
	a := newTestApp()
	canceled := false
	a.ghCancel = func() { canceled = true }
	result, _ := a.toMenu("")
	if !canceled {
		t.Error("toMenu must cancel a live gh context")
	}
	if asApp(t, result).ghCancel != nil {
		t.Error("ghCancel must be cleared after toMenu")
	}
}

func TestAppForwardSize_PhaseGHAuth(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	if waiting.gh == nil {
		t.Fatal("precondition: gh child must be set")
	}
	result, _ := waiting.forwardSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	if result == nil {
		t.Fatal("forwardSize(phaseGHAuth) returned nil model")
	}
}

func TestAppView_PhaseFormWithForm(t *testing.T) {
	a := newTestApp()
	a.form = newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	a.phase = phaseForm
	v := a.View()
	if string(v.Content) == "" {
		t.Error("View(phaseForm) with a form should render the form content")
	}
}

type fakeListItem struct{}

func (fakeListItem) FilterValue() string { return "x" }

func TestMenuDelegateRender_NonItemEarlyReturn(t *testing.T) {
	d := delegate{}
	d.Render(io.Discard, list.Model{}, 0, fakeListItem{})
}

func TestApplyHarness_Reinstall_FirstCallErrors(t *testing.T) {
	errBoom := errors.New("write marker boom")
	calls := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			calls++
			return "", errBoom
		},
	}
	if err := applyHarness(context.Background(), r, "reinstall"); !errors.Is(err, errBoom) {
		t.Errorf("applyHarness(reinstall) first-call error = %v, want boom", err)
	}
	if calls != 1 {
		t.Errorf("reinstall made %d container calls, want exactly 1 — it must stop after the marker write fails", calls)
	}
}
