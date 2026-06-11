package provision

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
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
			name: "seed-managed key (hooks) always wins",
			dest: map[string]any{"hooks": map[string]any{"old": "v"}},
			seed: map[string]any{"hooks": map[string]any{"new": "v"}},
			want: map[string]any{"hooks": map[string]any{"new": "v"}},
		},
		{
			name: "seed-managed key (statusLine) always wins",
			dest: map[string]any{"statusLine": "old"},
			seed: map[string]any{"statusLine": "new"},
			want: map[string]any{"statusLine": "new"},
		},
		{
			name: "seed-managed key (env) always wins",
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
			name: "empty dest — seed-owned keys come from seed",
			dest: map[string]any{},
			seed: map[string]any{"hooks": map[string]any{"x": "v"}},
			want: map[string]any{"hooks": map[string]any{"x": "v"}},
		},
		{
			name: "empty dest — user-owned key added from seed",
			dest: map[string]any{},
			seed: map[string]any{"theme": "dark"},
			want: map[string]any{"theme": "dark"},
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
			if !mapsEqual(got, tc.want) {
				t.Errorf("mergeSettings() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMergeSettings_ManagedKeys_UserEditsScalarSurvivesRestart(t *testing.T) {
	dest := map[string]any{
		"theme":               "dark",
		"includeCoAuthoredBy": true,
		"hooks":               map[string]any{"old": "hook"},
	}
	seed := map[string]any{
		"theme":               "auto",
		"includeCoAuthoredBy": false,
		"hooks":               map[string]any{"new": "hook"},
	}
	got := mergeSettings(dest, seed)
	if got["theme"] != "dark" {
		t.Errorf("theme should be user value 'dark', got %v", got["theme"])
	}
	if got["includeCoAuthoredBy"] != true {
		t.Errorf("includeCoAuthoredBy should be user value true, got %v", got["includeCoAuthoredBy"])
	}
	hooks, _ := got["hooks"].(map[string]any)
	if hooks == nil || hooks["new"] != "hook" {
		t.Errorf("hooks should be seed value (managed key), got %v", got["hooks"])
	}
	if _, hasOld := hooks["old"]; hasOld {
		t.Error("hooks must be fully replaced from seed (not merged); old key must not remain")
	}
}

func TestMergeSettingsIntegerPreserved(t *testing.T) {
	destJSON := `{"timeout": 15, "other": "val"}`
	seedJSON := `{"extra": "x"}`

	var dest, seed map[string]any
	dec := json.NewDecoder(jsonReader(destJSON))
	dec.UseNumber()
	if err := dec.Decode(&dest); err != nil {
		t.Fatal(err)
	}
	dec2 := json.NewDecoder(jsonReader(seedJSON))
	dec2.UseNumber()
	if err := dec2.Decode(&seed); err != nil {
		t.Fatal(err)
	}

	merged := mergeSettings(dest, seed)
	n, ok := merged["timeout"].(json.Number)
	if !ok {
		t.Fatalf("timeout should be json.Number, got %T", merged["timeout"])
	}
	if n.String() != "15" {
		t.Errorf("timeout = %q, want %q", n.String(), "15")
	}
}

func TestEnsureSettings_SandboxDropped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	seedDir := filepath.Join(tmp, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seedFile := filepath.Join(seedDir, "settings.json")
	seedContent := map[string]any{
		"sandbox": map[string]any{"enabled": true},
		"theme":   "dark",
	}
	writeTestJSON(t, seedFile, seedContent)

	destFile := filepath.Join(tmp, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(destFile), 0o755); err != nil {
		t.Fatal(err)
	}
	destContent := map[string]any{
		"sandbox":   map[string]any{"enabled": false},
		"user-only": "kept",
		"theme":     "light",
	}
	writeTestJSON(t, destFile, destContent)

	cfg := config.New(seedDir)
	if err := EnsureSettings(cfg); err != nil {
		t.Fatalf("EnsureSettings: %v", err)
	}

	result, err := readJSON(destFile)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	if _, hasSandbox := result["sandbox"]; hasSandbox {
		t.Error("sandbox key should be deleted after merge")
	}
	if v, ok := result["user-only"]; !ok || v != "kept" {
		t.Errorf("user-only key should be preserved, got %v", v)
	}
	if v, ok := result["theme"]; !ok || v != "light" {
		t.Errorf("theme should be user value 'light' (user-owned key), got %v", v)
	}
}

func TestEnsureSettings_SeedCopiedWhenNoDestination(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	seedDir := filepath.Join(tmp, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seedFile := filepath.Join(seedDir, "settings.json")
	seedContent := map[string]any{"sandbox": map[string]any{"x": 1}, "theme": "auto"}
	writeTestJSON(t, seedFile, seedContent)

	cfg := config.New(seedDir)
	if err := EnsureSettings(cfg); err != nil {
		t.Fatalf("EnsureSettings: %v", err)
	}

	destFile := filepath.Join(tmp, ".claude", "settings.json")
	result, err := readJSON(destFile)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	if _, hasSandbox := result["sandbox"]; !hasSandbox {
		t.Error("direct seed copy should preserve sandbox key")
	}
}

func writeTestJSON(t *testing.T, path string, m map[string]any) {
	t.Helper()
	if err := WriteJSON(path, m); err != nil {
		t.Fatalf("WriteJSON(%s): %v", path, err)
	}
}

func TestWriteJSON_RenameFails_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "target")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err := WriteJSON(path, map[string]any{"k": "v"})
	if err == nil {
		t.Error("WriteJSON must return error when target path is a directory (Rename fails)")
	}
}

func jsonReader(s string) *strings.Reader { return strings.NewReader(s) }

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		switch av2 := av.(type) {
		case map[string]any:
			bv2, ok := bv.(map[string]any)
			if !ok {
				return false
			}
			if !mapsEqual(av2, bv2) {
				return false
			}
		default:

			if fmt.Sprintf("%v", av) != fmt.Sprintf("%v", bv) {
				return false
			}
		}
	}
	return true
}
