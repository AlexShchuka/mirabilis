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
			name: "seed wins on leaf conflict",
			dest: map[string]any{"key": "dest-val"},
			seed: map[string]any{"key": "seed-val"},
			want: map[string]any{"key": "seed-val"},
		},
		{
			name: "nested object recursive merge",
			dest: map[string]any{
				"outer": map[string]any{"a": "1", "b": "dest-b"},
			},
			seed: map[string]any{
				"outer": map[string]any{"b": "seed-b", "c": "3"},
			},
			want: map[string]any{
				"outer": map[string]any{"a": "1", "b": "seed-b", "c": "3"},
			},
		},
		{
			name: "arrays replaced not concatenated",
			dest: map[string]any{"arr": []any{"x", "y"}},
			seed: map[string]any{"arr": []any{"z"}},
			want: map[string]any{"arr": []any{"z"}},
		},
		{
			name: "dest-only keys preserved",
			dest: map[string]any{"user-only": "kept", "shared": "dest"},
			seed: map[string]any{"shared": "seed"},
			want: map[string]any{"user-only": "kept", "shared": "seed"},
		},
		{
			name: "empty seed",
			dest: map[string]any{"a": "1"},
			seed: map[string]any{},
			want: map[string]any{"a": "1"},
		},
		{
			name: "empty dest",
			dest: map[string]any{},
			seed: map[string]any{"a": "1"},
			want: map[string]any{"a": "1"},
		},
		{
			name: "object in seed replaces non-object in dest",
			dest: map[string]any{"k": "string"},
			seed: map[string]any{"k": map[string]any{"nested": "v"}},
			want: map[string]any{"k": map[string]any{"nested": "v"}},
		},
		{
			name: "non-object in seed replaces object in dest",
			dest: map[string]any{"k": map[string]any{"nested": "v"}},
			seed: map[string]any{"k": "flat"},
			want: map[string]any{"k": "flat"},
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
	if v, ok := result["theme"]; !ok || v != "dark" {
		t.Errorf("theme should be seed value 'dark', got %v", v)
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
	if err := writeJSON(path, m); err != nil {
		t.Fatalf("writeJSON(%s): %v", path, err)
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
