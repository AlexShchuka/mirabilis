package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadPluginCatalog(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		noConfig bool
		want     []string
	}{
		{
			name:    "empty file returns nil",
			content: "",
			want:    nil,
		},
		{
			name:    "skips blank lines and comments",
			content: "# a comment\n\nplugin-a\nplugin-b\n# another comment\nplugin-c\n",
			want:    []string{"plugin-a", "plugin-b", "plugin-c"},
		},
		{
			name:     "no config dir returns nil",
			noConfig: true,
			want:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.noConfig {
				if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
					t.Fatal(err)
				}
				if tt.content != "" {
					if err := os.WriteFile(filepath.Join(dir, "config", "plugins.txt"), []byte(tt.content), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			got := ReadPluginCatalog(dir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadPluginCatalog = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadPluginsDisabled(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "no .env returns nil",
			content: "",
			want:    nil,
		},
		{
			name:    "PLUGINS_DISABLED empty returns nil",
			content: "PLUGINS_DISABLED=\n",
			want:    nil,
		},
		{
			name:    "single plugin",
			content: "PLUGINS_DISABLED=plugin-a\n",
			want:    []string{"plugin-a"},
		},
		{
			name:    "multiple plugins",
			content: "STACKS=go\nPLUGINS_DISABLED=plugin-a,plugin-b,plugin-c\n",
			want:    []string{"plugin-a", "plugin-b", "plugin-c"},
		},
		{
			name:    "trims spaces",
			content: "PLUGINS_DISABLED= plugin-a , plugin-b \n",
			want:    []string{"plugin-a", "plugin-b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.content != "" {
				if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := ReadPluginsDisabled(dir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadPluginsDisabled = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWritePluginsDisabled(t *testing.T) {
	t.Run("creates .env with PLUGINS_DISABLED", func(t *testing.T) {
		dir := t.TempDir()
		if err := WritePluginsDisabled(dir, []string{"plugin-a", "plugin-b"}); err != nil {
			t.Fatal(err)
		}
		got := ReadPluginsDisabled(dir)
		want := []string{"plugin-a", "plugin-b"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("preserves existing STACKS line", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("STACKS=go,rust\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := WritePluginsDisabled(dir, []string{"plugin-x"}); err != nil {
			t.Fatal(err)
		}
		stacks, _ := ReadStacks(dir)
		if stacks != "go,rust" {
			t.Errorf("STACKS was clobbered: got %q, want %q", stacks, "go,rust")
		}
		got := ReadPluginsDisabled(dir)
		if !reflect.DeepEqual(got, []string{"plugin-x"}) {
			t.Errorf("PLUGINS_DISABLED = %v, want [plugin-x]", got)
		}
	})

	t.Run("overwrites existing PLUGINS_DISABLED line", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PLUGINS_DISABLED=old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := WritePluginsDisabled(dir, []string{"new"}); err != nil {
			t.Fatal(err)
		}
		got := ReadPluginsDisabled(dir)
		if !reflect.DeepEqual(got, []string{"new"}) {
			t.Errorf("got %v, want [new]", got)
		}
	})

	t.Run("empty disabled list writes empty CSV", func(t *testing.T) {
		dir := t.TempDir()
		if err := WritePluginsDisabled(dir, nil); err != nil {
			t.Fatal(err)
		}
		got := ReadPluginsDisabled(dir)
		if got != nil {
			t.Errorf("got %v, want nil for empty disabled list", got)
		}
	})
}
