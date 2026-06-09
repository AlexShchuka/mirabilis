package main

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newTestApp() appModel {
	return newApp(context.Background(), fakeRunner{}, Status{})
}

func asApp(t *testing.T, m tea.Model) appModel {
	t.Helper()
	a, ok := m.(appModel)
	if !ok {
		t.Fatal("model is not appModel")
	}
	return a
}

func TestAppRouteLaunch(t *testing.T) {
	a := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	if a.phase != phasePipeline {
		t.Errorf("phase = %v, want pipeline", a.phase)
	}
	if a.pipe == nil {
		t.Error("pipeline not created on launch")
	}
}

func TestAppRoutePluginsNoCatalog(t *testing.T) {
	a := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"plugins"}))
	if a.phase != phaseMenu {
		t.Errorf("phase = %v, want menu", a.phase)
	}
	if a.notice == "" {
		t.Error("expected a notice when the plugin catalog is unavailable")
	}
}

func TestAppPipelineDoneSuccessHandsOff(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, pipelineDoneMsg{failed: false}))
	if !a.handoff {
		t.Error("successful pipeline should set handoff")
	}
}

func TestAppPipelineDoneFailureStays(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, pipelineDoneMsg{failed: true}))
	if a.handoff {
		t.Error("failed pipeline must not hand off")
	}
	if !a.pipeEnd {
		t.Error("failed pipeline should set pipeEnd")
	}
}

func TestAppNeedGHSwitchesToGHAuth(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	a := asApp(t, mustUpdate(launched, needGHMsg{name: "gh"}))
	if a.phase != phaseGHAuth {
		t.Errorf("phase = %v, want ghauth", a.phase)
	}
	if a.gh == nil || a.pendGH != "gh" {
		t.Errorf("gh child not set up (pendGH=%q)", a.pendGH)
	}
}

func TestAppGHDoneResumesPipeline(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, needGHMsg{name: "gh"}))
	a := asApp(t, mustUpdate(waiting, ghDoneMsg{err: nil}))
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
