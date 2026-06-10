package app

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/ghauth"
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/ui"
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

func TestAppGHAuthEscGoesBackToMenu(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	if waiting.phase != phaseGHAuth {
		t.Fatalf("phase = %v, want ghauth", waiting.phase)
	}
	if waiting.ghCancel == nil {
		t.Fatal("ghCancel must be set while in phaseGHAuth")
	}

	canceled := false
	waiting.ghCancel = func() { canceled = true }

	a := asApp(t, mustUpdate(waiting, tea.KeyPressMsg{Code: tea.KeyEscape}))
	if !canceled {
		t.Error("esc in phaseGHAuth must call ghCancel")
	}
	if a.phase != phaseMenu {
		t.Errorf("phase = %v, want menu after esc in ghauth", a.phase)
	}
	if a.ghCancel != nil {
		t.Error("ghCancel must be nil after cancellation")
	}
}

func TestAppGHAuthNilGHGuarded(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	waiting.gh = nil

	a := asApp(t, mustUpdate(waiting, tea.KeyPressMsg{Code: 'x'}))
	if a.phase != phaseGHAuth {
		t.Errorf("phase = %v, want ghauth — nil gh must not crash", a.phase)
	}
}

func TestAppGHDoneCancelsGHCtx(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	if waiting.ghCancel == nil {
		t.Fatal("ghCancel not set")
	}

	canceled := false
	waiting.ghCancel = func() { canceled = true }

	asApp(t, mustUpdate(waiting, ghauth.DoneMsg{Err: nil}))
	if !canceled {
		t.Error("ghCancel must be called when ghauth.DoneMsg is received")
	}
}

func mustUpdate(m tea.Model, msg tea.Msg) tea.Model {
	next, _ := m.Update(msg)
	return next
}

func TestAppInit_ReturnsMenuInit(t *testing.T) {
	a := newTestApp()
	cmd := a.Init()
	if cmd != nil {
		t.Error("appModel.Init() should return nil (menu.Init returns nil)")
	}
}

func TestAppView_PhaseMenu(t *testing.T) {
	a := newTestApp()
	a.notice = "test-notice"
	v := a.View()
	content := string(v.Content)
	if !strings.Contains(content, ui.OffStyle.Render("test-notice")) {
		t.Errorf("View(phaseMenu) missing notice rendering, got:\n%s", content)
	}
	if !v.AltScreen {
		t.Error("View should always have AltScreen=true")
	}
}

func TestAppView_PhaseMenu_NoNotice(t *testing.T) {
	a := newTestApp()
	v := a.View()
	if !v.AltScreen {
		t.Error("View should have AltScreen=true")
	}
}

func TestAppView_PhasePipelineNilPipe(t *testing.T) {
	a := newTestApp()
	a.phase = phasePipeline
	a.pipe = nil
	v := a.View()
	content := string(v.Content)
	if content != "" {
		t.Errorf("View(phasePipeline) with nil pipe should return empty content, got %q", content)
	}
}

func TestAppView_PhasePipelineWithPipe(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	v := launched.View()
	content := string(v.Content)
	if !strings.Contains(content, ui.PipelineTitle) {
		t.Errorf("View(phasePipeline) missing pipeline title, got:\n%s", content)
	}
}

func TestAppView_PhaseFormNilForm(t *testing.T) {
	a := newTestApp()
	a.phase = phaseForm
	a.form = nil
	v := a.View()
	content := string(v.Content)
	if content != "" {
		t.Errorf("View(phaseForm) with nil form should be empty, got %q", content)
	}
}

func TestAppView_PhaseGHAuthNilGH(t *testing.T) {
	a := newTestApp()
	a.phase = phaseGHAuth
	a.gh = nil
	v := a.View()
	content := string(v.Content)
	if content != "" {
		t.Errorf("View(phaseGHAuth) with nil gh should be empty, got %q", content)
	}
}

func TestAppView_PhaseGHAuthWithGH(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	waiting := asApp(t, mustUpdate(launched, pipeline.NeedGHMsg{Name: "gh"}))
	v := waiting.View()
	content := string(v.Content)
	if !strings.Contains(content, ui.GHAuthTitle) {
		t.Errorf("View(phaseGHAuth) missing GHAuthTitle, got:\n%s", content)
	}
}

func TestAppToMenu_ResetsChildren(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	if launched.phase != phasePipeline {
		t.Fatalf("precondition: phase should be pipeline, got %v", launched.phase)
	}
	if launched.pipe == nil {
		t.Fatal("precondition: pipe should be non-nil")
	}
	a, _ := launched.toMenu("test-notice")
	result := a.(appModel)
	if result.phase != phaseMenu {
		t.Errorf("toMenu: phase = %v, want menu", result.phase)
	}
	if result.pipe != nil {
		t.Error("toMenu: pipe should be nil")
	}
	if result.form != nil {
		t.Error("toMenu: form should be nil")
	}
	if result.gh != nil {
		t.Error("toMenu: gh should be nil")
	}
	if result.notice != "test-notice" {
		t.Errorf("toMenu: notice = %q, want test-notice", result.notice)
	}
	if result.pipeEnd {
		t.Error("toMenu: pipeEnd should be false")
	}
}

func TestAppForwardSize_PerPhase(t *testing.T) {
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 30}

	t.Run("phaseMenu", func(t *testing.T) {
		a := newTestApp()
		a.w, a.h = 80, 24
		result := asApp(t, mustUpdate(a, sizeMsg))
		if result.w != 100 || result.h != 30 {
			t.Errorf("w=%d h=%d, want 100 30", result.w, result.h)
		}
	})

	t.Run("phasePipeline_nilPipe", func(t *testing.T) {
		a := newTestApp()
		a.phase = phasePipeline
		a.pipe = nil
		result, _ := a.forwardSize(sizeMsg)
		if result == nil {
			t.Fatal("forwardSize returned nil")
		}
	})

	t.Run("phasePipeline_withPipe", func(t *testing.T) {
		a := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
		result, _ := a.forwardSize(sizeMsg)
		if result == nil {
			t.Fatal("forwardSize returned nil")
		}
	})

	t.Run("phaseGHAuth_nilGH", func(t *testing.T) {
		a := newTestApp()
		a.phase = phaseGHAuth
		a.gh = nil
		result, _ := a.forwardSize(sizeMsg)
		if result == nil {
			t.Fatal("forwardSize returned nil")
		}
	})
}

func TestAppForwardToPipe_NilGuard(t *testing.T) {
	a := newTestApp()
	a.pipe = nil
	result, cmd := a.forwardToPipe(pipeline.CheckedMsg{})
	if result == nil {
		t.Fatal("forwardToPipe with nil pipe returned nil model")
	}
	if cmd != nil {
		t.Error("forwardToPipe with nil pipe should return nil cmd")
	}
}

func TestAppRouteVSCode_ReturnsCmd(t *testing.T) {
	dir := makeDockerShim(t, "exit 0")
	codeDir := makeCodeShim(t, "exit 0")
	prependTestPath(t, dir, codeDir)

	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
	}
	a := newApp(context.Background(), r, provision.Status{})
	_, cmd := a.route("vscode")
	if cmd == nil {
		t.Error("route(vscode) should return non-nil cmd")
	}
}

func TestAppRouteHarness_OpensForm(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "skip", nil
		},
	}
	a := newApp(context.Background(), r, provision.Status{})
	result := asApp(t, mustUpdate(a, menuChoiceMsg{"harness"}))
	if result.phase != phaseForm {
		t.Errorf("route(harness) phase = %v, want form", result.phase)
	}
	if result.form == nil {
		t.Error("route(harness) form should be non-nil")
	}
}

func TestAppRouteUnknown_NoChange(t *testing.T) {
	a := newTestApp()
	result := asApp(t, mustUpdate(a, menuChoiceMsg{"unknown-action"}))
	if result.phase != phaseMenu {
		t.Errorf("route(unknown) phase = %v, want menu", result.phase)
	}
}

func TestAppForwardSize_PhaseForm(t *testing.T) {
	a := newTestApp()
	a.form = newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	a.phase = phaseForm
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	result, _ := a.forwardSize(sizeMsg)
	if result == nil {
		t.Fatal("forwardSize(phaseForm) returned nil")
	}
}

func TestAppPipelineEndAnyKeyReturnsMenu(t *testing.T) {
	launched := asApp(t, mustUpdate(newTestApp(), menuChoiceMsg{"launch"}))
	failed := asApp(t, mustUpdate(launched, pipeline.DoneMsg{Failed: true}))
	if !failed.pipeEnd {
		t.Fatal("precondition: pipeEnd should be true")
	}
	back := asApp(t, mustUpdate(failed, tea.KeyPressMsg{Code: 'x', Text: "x"}))
	if back.phase != phaseMenu {
		t.Errorf("any key after pipeEnd: phase = %v, want menu", back.phase)
	}
}

func makeDockerShim(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := dir + "/docker"
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeCodeShim(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := dir + "/code"
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func prependTestPath(t *testing.T, dirs ...string) {
	t.Helper()
	base := os.Getenv("PATH")
	prefix := ""
	for _, d := range dirs {
		if prefix != "" {
			prefix += ":"
		}
		prefix += d
	}
	t.Setenv("PATH", prefix+":"+base)
}
