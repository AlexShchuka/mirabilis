package provision

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMergeSettings(t *testing.T) {
	tests := []struct {
		dest map[string]any
		seed map[string]any
		want map[string]any
		name string
	}{
		{
			name: "user key preserved when seed has same key",
			dest: map[string]any{"theme": "light"},
			seed: map[string]any{"theme": "dark"},
			want: map[string]any{"theme": "light"},
		},
		{
			name: "seed-managed key hooks always wins",
			dest: map[string]any{"hooks": map[string]any{"old": "v"}},
			seed: map[string]any{"hooks": map[string]any{"new": "v"}},
			want: map[string]any{"hooks": map[string]any{"new": "v"}},
		},
		{
			name: "seed-managed key statusLine always wins",
			dest: map[string]any{"statusLine": "old"},
			seed: map[string]any{"statusLine": "new"},
			want: map[string]any{"statusLine": "new"},
		},
		{
			name: "seed-managed key env always wins",
			dest: map[string]any{"env": map[string]any{"K": "old"}},
			seed: map[string]any{"env": map[string]any{"K": "new"}},
			want: map[string]any{"env": map[string]any{"K": "new"}},
		},
		{
			name: "seed adds new user-owned key not present in dest",
			dest: map[string]any{"existing": "kept"},
			seed: map[string]any{"newkey": "added"},
			want: map[string]any{"existing": "kept", "newkey": "added"},
		},
		{
			name: "dest-only keys preserved",
			dest: map[string]any{"user-only": "kept", "shared": "dest"},
			seed: map[string]any{"shared": "seed"},
			want: map[string]any{"user-only": "kept", "shared": "dest"},
		},
		{
			name: "empty seed",
			dest: map[string]any{"a": "1"},
			seed: map[string]any{},
			want: map[string]any{"a": "1"},
		},
		{
			name: "empty dest seed-owned keys come from seed",
			dest: map[string]any{},
			seed: map[string]any{"hooks": map[string]any{"x": "v"}},
			want: map[string]any{"hooks": map[string]any{"x": "v"}},
		},
		{
			name: "nested merge only for non-managed keys that both have as maps",
			dest: map[string]any{"outer": map[string]any{"a": "1", "b": "dest-b"}},
			seed: map[string]any{"outer": map[string]any{"b": "seed-b", "c": "3"}},
			want: map[string]any{"outer": map[string]any{"a": "1", "b": "dest-b", "c": "3"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeSettings(tc.dest, tc.seed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("mergeSettings() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMergeSettingsIntegerPreserved(t *testing.T) {
	var dest, seed map[string]any
	dec := json.NewDecoder(strings.NewReader(`{"timeout": 15, "other": "val"}`))
	dec.UseNumber()
	if err := dec.Decode(&dest); err != nil {
		t.Fatal(err)
	}
	dec = json.NewDecoder(strings.NewReader(`{"extra": "x"}`))
	dec.UseNumber()
	if err := dec.Decode(&seed); err != nil {
		t.Fatal(err)
	}
	merged := mergeSettings(dest, seed)
	n, ok := merged["timeout"].(json.Number)
	if !ok {
		t.Fatalf("timeout should be json.Number, got %T", merged["timeout"])
	}
	if n.String() != "15" {
		t.Errorf("timeout = %q, want 15", n.String())
	}
}

func TestSettingsStepMergeDropsSandbox(t *testing.T) {
	d, _ := testDeps(t)
	mustWriteJSON(t, d.Cfg.SettingsSeed(), map[string]any{
		"sandbox": map[string]any{"enabled": true},
		"theme":   "dark",
		"hooks":   map[string]any{"new": "v"},
	})
	mustWriteJSON(t, d.settingsPath(), map[string]any{
		"sandbox":   map[string]any{"enabled": false},
		"user-only": "kept",
		"theme":     "light",
	})
	step := &settingsStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := mustReadJSON(t, d.settingsPath())
	if _, has := got["sandbox"]; has {
		t.Error("sandbox key should be deleted after merge")
	}
	if got["user-only"] != "kept" {
		t.Errorf("user-only = %v, want kept", got["user-only"])
	}
	if got["theme"] != "light" {
		t.Errorf("theme = %v, want user value light", got["theme"])
	}
	hooks, _ := got["hooks"].(map[string]any)
	if hooks == nil || hooks["new"] != "v" {
		t.Errorf("hooks = %v, want seed value", got["hooks"])
	}
}

func TestSettingsStepSeedCopiedWhenNoDestination(t *testing.T) {
	d, _ := testDeps(t)
	mustWriteJSON(t, d.Cfg.SettingsSeed(), map[string]any{
		"sandbox": map[string]any{"x": "1"},
		"theme":   "auto",
	})
	step := &settingsStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := mustReadJSON(t, d.settingsPath())
	if _, has := got["sandbox"]; !has {
		t.Error("direct seed copy should preserve sandbox key")
	}
	if !exists(filepath.Join(d.claudeDir(), "xdg-data")) {
		t.Error("xdg-data dir should be created")
	}
}

func TestSettingsStepCheck(t *testing.T) {
	d, _ := testDeps(t)
	mustWriteJSON(t, d.Cfg.SettingsSeed(), map[string]any{
		"hooks":      map[string]any{},
		"statusLine": "s",
		"env":        map[string]any{},
	})
	step := &settingsStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false before dirs exist")
	}
	if err := runStep(t, step); err != nil {
		t.Fatal(err)
	}
	if !checkStep(t, step) {
		t.Error("check should be true after run")
	}
	mustWriteJSON(t, d.settingsPath(), map[string]any{"hooks": map[string]any{}})
	if checkStep(t, step) {
		t.Error("check should be false when a seed-managed key is missing from dest")
	}
}

func TestThemeStepAppliesOnlyWhenThemeFilePresent(t *testing.T) {
	d, _ := testDeps(t)
	mustWriteJSON(t, d.settingsPath(), map[string]any{"a": "1"})
	step := &themeStep{d: d}
	if !checkStep(t, step) {
		t.Error("check should be true without a theme file")
	}
	if err := runStep(t, step); err != nil {
		t.Fatal(err)
	}
	if _, has := mustReadJSON(t, d.settingsPath())["theme"]; has {
		t.Error("run without theme file must not touch settings")
	}

	mustWrite(t, d.themePath(), "tokyo-night\r\n")
	if checkStep(t, step) {
		t.Error("check should be false when theme file differs from settings")
	}
	if err := runStep(t, step); err != nil {
		t.Fatal(err)
	}
	got := mustReadJSON(t, d.settingsPath())
	if got["theme"] != "tokyo-night" {
		t.Errorf("theme = %v, want tokyo-night with trailing newline trimmed", got["theme"])
	}
	if got["a"] != "1" {
		t.Errorf("unrelated key lost: %v", got["a"])
	}
	if !checkStep(t, step) {
		t.Error("check should be true after theme applied")
	}
}

func TestThemeStepNoSettingsFile(t *testing.T) {
	d, _ := testDeps(t)
	mustWrite(t, d.themePath(), "dark\n")
	step := &themeStep{d: d}
	if !checkStep(t, step) {
		t.Error("check should be true when settings.json is absent")
	}
	if err := runStep(t, step); err != nil {
		t.Fatal(err)
	}
	if exists(d.settingsPath()) {
		t.Error("run must not create settings.json")
	}
}
