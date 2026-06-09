package app

import (
	"context"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/ghauth"
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func newTestApp() appModel {
	return newApp(context.Background(), &runner.FakeRunner{}, provision.Status{})
}

func asApp(t *testing.T, m tea.Model) appModel {
	t.Helper()
	a, ok := m.(appModel)
	if !ok {
		t.Fatal("model is not appModel")
	}
	return a
}

func TestAppRouteLaunchNoCatalogStartsPipeline(t *testing.T) {
	a := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	if a.phase != phasePipeline {
		t.Errorf("phase = %v, want pipeline", a.phase)
	}
	if a.pipe == nil {
		t.Error("pipeline not created on launch when catalogs are empty")
	}
}

func TestAppRouteLaunchWithCatalogOpensForm(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/config/stacks.txt", []byte("rust\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{RepoVal: dir}
	a := newApp(context.Background(), r, provision.Status{})
	a2 := asApp(t, mustUpdate(a, menuChoiceMsg{"launch"}))
	if a2.phase != phaseForm {
		t.Errorf("phase = %v, want form when catalog is non-empty", a2.phase)
	}
	if a2.form == nil {
		t.Error("form should be set when launch catalog exists")
	}
}

func TestAppLaunchReadyMsgStartsPipeline(t *testing.T) {
	a := newApp(context.Background(), &runner.FakeRunner{}, provision.Status{})
	a2 := asApp(t, mustUpdate(a, launchReadyMsg{}))
	if a2.phase != phasePipeline {
		t.Errorf("phase = %v, want pipeline after launchReadyMsg", a2.phase)
	}
	if a2.pipe == nil {
		t.Error("pipeline not created after launchReadyMsg")
	}
	if a2.form != nil {
		t.Error("form should be cleared after launchReadyMsg")
	}
}

func TestAppPipelineDoneSuccessHandsOff(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, pipeline.DoneMsg{Failed: false}))
	if !a.handoff {
		t.Error("successful pipeline should set handoff")
	}
}

func TestAppPipelineDoneFailureStays(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, pipeline.DoneMsg{Failed: true}))
	if a.handoff {
		t.Error("failed pipeline must not hand off")
	}
	if !a.pipeEnd {
		t.Error("failed pipeline should set pipeEnd")
	}
}

func TestAppNeedGHSwitchesToGHAuth(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	if a.phase != phaseGHAuth {
		t.Errorf("phase = %v, want ghauth", a.phase)
	}
	if a.gh == nil || a.pendGH != "gh" {
		t.Errorf("gh child not set up (pendGH=%q)", a.pendGH)
	}
}

func TestAppGHDoneResumesPipeline(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	a := asApp(t, mustUpdate(waiting, ghauth.DoneMsg{Err: nil}))
	if a.phase != phasePipeline {
		t.Errorf("phase = %v, want pipeline after gh done", a.phase)
	}
}

func TestAppEscCancelsPipeline(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	if launched.pipeCancel == nil {
		t.Fatal("launch should install a cancel func for the pipeline context")
	}
	canceled := false
	launched.pipeCancel = func() { canceled = true }

	a := asApp(t, mustUpdate(launched, tea.KeyPressMsg{Code: tea.KeyEscape}))
	if !canceled {
		t.Error("esc should cancel the running pipeline context")
	}
	if a.phase != phaseMenu {
		t.Errorf("phase = %v, want menu after esc", a.phase)
	}
}

func TestAppForwardsPipelineMsgsDuringGHAuth(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	if waiting.phase != phaseGHAuth {
		t.Fatalf("phase = %v, want ghauth", waiting.phase)
	}

	a := asApp(t, mustUpdate(waiting, pipeline.RanMsg{Name: "harness", Err: nil}))
	if a.phase != phaseGHAuth {
		t.Errorf("phase = %v, want ghauth — forwarding a step result must not leave the gh screen", a.phase)
	}
}

func TestAppRouteReset(t *testing.T) {
	a := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"reset"}))
	if a.phase != phaseForm {
		t.Errorf("phase = %v, want form", a.phase)
	}
	if a.form == nil {
		t.Error("reset should open a confirm form")
	}
}

func TestAppQuitFromMenu(t *testing.T) {
	_, cmd := newTestApp().Update(menuChoiceMsg{"quit"})
	if cmd == nil {
		t.Fatal("quit returned nil cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("quit action should issue tea.Quit")
	}
}

func TestAppCtrlCQuits(t *testing.T) {
	_, cmd := newTestApp().Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c returned nil cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should issue tea.Quit")
	}
}

func mustUpdate(m tea.Model, msg tea.Msg) tea.Model {
	next, _ := m.Update(msg)
	return next
}
