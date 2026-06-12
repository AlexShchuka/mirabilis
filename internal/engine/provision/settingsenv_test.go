package provision

import (
	"testing"
)

func TestSettingsEnvStepSetsBaseURLAlways(t *testing.T) {
	d, _ := testDeps(t)
	step := &settingsEnvStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false before settings exist")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	env, _ := mustReadJSON(t, d.settingsPath())[settingsEnvKey].(map[string]any)
	if env == nil {
		t.Fatal("env block missing")
	}
	if env[settingsURLKey] != settingsBaseURL {
		t.Errorf("base url = %v, want %s", env[settingsURLKey], settingsBaseURL)
	}
	if _, has := env[settingsTokenKey]; has {
		t.Error("token must be absent when SessionKey is empty")
	}
	if !checkStep(t, step) {
		t.Error("check should be true after run")
	}
}

func TestSettingsEnvStepSetsTokenWhenSessionKeyPresent(t *testing.T) {
	d, _ := testDeps(t)
	d.SessionKey = "sk-session"
	mustWriteJSON(t, d.settingsPath(), map[string]any{
		"theme": "dark",
		settingsEnvKey: map[string]any{
			"UNRELATED":      "kept",
			settingsTokenKey: "stale",
		},
	})
	step := &settingsEnvStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false while the token is stale")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	m := mustReadJSON(t, d.settingsPath())
	env, _ := m[settingsEnvKey].(map[string]any)
	if env[settingsTokenKey] != "sk-session" {
		t.Errorf("token = %v, want sk-session", env[settingsTokenKey])
	}
	if env[settingsURLKey] != settingsBaseURL {
		t.Errorf("base url = %v, want %s", env[settingsURLKey], settingsBaseURL)
	}
	if env["UNRELATED"] != "kept" {
		t.Errorf("unrelated env key lost: %v", env["UNRELATED"])
	}
	if m["theme"] != "dark" {
		t.Errorf("unrelated top-level key lost: %v", m["theme"])
	}
	if !checkStep(t, step) {
		t.Error("check should be true after run")
	}
}
