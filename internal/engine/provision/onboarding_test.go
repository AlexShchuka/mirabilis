package provision

import (
	"testing"
)

func TestOnboardingRunThenCheckTrue(t *testing.T) {
	d, _ := testDeps(t)
	step := &onboardingStep{d: d}

	if checkStep(t, step) {
		t.Fatal("Check() = true before Run(), want false")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !checkStep(t, step) {
		t.Fatal("Check() = false after Run(), want true (INV-GATEFREE)")
	}
}

func TestOnboardingFullBeltPresent(t *testing.T) {
	d, _ := testDeps(t)
	step := &onboardingStep{d: d}

	if err := runStep(t, step); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	m := mustReadJSON(t, d.claudeJSONPath())
	for k, want := range onboardingBeltKeys {
		if m[k] != want {
			t.Errorf("belt key %q = %v, want %v (INV-GATEFREE)", k, m[k], want)
		}
	}
	if m["skipDangerousModePermissionPrompt"] != true {
		t.Error("skipDangerousModePermissionPrompt not set in .claude.json (INV-GATEFREE)")
	}
	sm := mustReadJSON(t, d.settingsPath())
	if sm["skipDangerousModePermissionPrompt"] != true {
		t.Error("skipDangerousModePermissionPrompt not set in settings.json (INV-GATEFREE)")
	}
}

func TestOnboardingMergePreservesOtherKeys(t *testing.T) {
	d, _ := testDeps(t)
	step := &onboardingStep{d: d}

	mustWriteJSON(t, d.claudeJSONPath(), map[string]any{
		"unrelated": "keep-me",
		"projects": map[string]any{
			"/other": map[string]any{"hasTrustDialogAccepted": true},
		},
	})

	if err := runStep(t, step); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !checkStep(t, step) {
		t.Fatal("Check() = false after Run(), want true")
	}

	m := mustReadJSON(t, d.claudeJSONPath())
	if m["unrelated"] != "keep-me" {
		t.Errorf("unrelated key lost, got %v", m["unrelated"])
	}
	projects, ok := m["projects"].(map[string]any)
	if !ok {
		t.Fatal("projects is not map")
	}
	other, ok := projects["/other"].(map[string]any)
	if !ok {
		t.Fatal("projects['/other'] missing after merge")
	}
	if other["hasTrustDialogAccepted"] != true {
		t.Error("pre-existing project entry was overwritten")
	}
	ws, ok := projects[claudeWorkspaceDir].(map[string]any)
	if !ok {
		t.Fatal("workspace project entry missing")
	}
	if ws["hasTrustDialogAccepted"] != true {
		t.Error("workspace hasTrustDialogAccepted not set")
	}
}
