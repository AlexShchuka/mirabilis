package provision

import (
	"testing"
)

func TestOnboardingRunThenCheckTrue(t *testing.T) {
	d, _ := testDeps(t)
	step := &onboardingStep{d: d, version: "9.9.9"}

	if checkStep(t, step) {
		t.Fatal("Check() = true before Run(), want false")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !checkStep(t, step) {
		t.Fatal("Check() = false after Run(), want true")
	}
}

func TestOnboardingSeededKeysGate(t *testing.T) {
	d, _ := testDeps(t)
	step := &onboardingStep{d: d, version: "9.9.9"}

	if err := runStep(t, step); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	m := mustReadJSON(t, d.claudeJSONPath())

	if m["hasCompletedOnboarding"] != true {
		t.Error("hasCompletedOnboarding not set")
	}
	if m["skipDangerousModePermissionPrompt"] != true {
		t.Error("skipDangerousModePermissionPrompt not set — bypass-mode warning gate not auto-accepted")
	}
	if m["lastOnboardingVersion"] != "9.9.9" {
		t.Errorf("lastOnboardingVersion = %v, want 9.9.9", m["lastOnboardingVersion"])
	}
	projects, _ := m["projects"].(map[string]any)
	if projects == nil {
		t.Fatal("projects missing")
	}
	ws, _ := projects[claudeWorkspaceDir].(map[string]any)
	if ws == nil {
		t.Fatal("workspace project entry missing")
	}
	if ws["hasTrustDialogAccepted"] != true {
		t.Error("hasTrustDialogAccepted not set")
	}
	if ws["hasCompletedProjectOnboarding"] != true {
		t.Error("hasCompletedProjectOnboarding not set")
	}
}

func TestOnboardingCheckMissingSkipKeyFails(t *testing.T) {
	d, _ := testDeps(t)
	step := &onboardingStep{d: d, version: "9.9.9"}

	mustWriteJSON(t, d.claudeJSONPath(), map[string]any{
		"hasCompletedOnboarding": true,
		"projects": map[string]any{
			claudeWorkspaceDir: map[string]any{
				"hasTrustDialogAccepted":        true,
				"hasCompletedProjectOnboarding": true,
			},
		},
	})

	if checkStep(t, step) {
		t.Error("Check() = true without skipDangerousModePermissionPrompt — bypass gate not enforced")
	}
}

func TestOnboardingMergePreservesOtherKeys(t *testing.T) {
	d, _ := testDeps(t)
	step := &onboardingStep{d: d, version: "9.9.9"}

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
