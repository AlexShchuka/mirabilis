package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMemoryCategories_SharedCodebook(t *testing.T) {
	var found *MemoryCategory
	for i := range MemoryCategories {
		if MemoryCategories[i].Name == "shared-codebook" {
			found = &MemoryCategories[i]
			break
		}
	}
	if found == nil {
		t.Fatal("shared-codebook category not found in MemoryCategories")
	}
	if found.MemoryType != "semantic" {
		t.Errorf("shared-codebook MemoryType = %q, want semantic", found.MemoryType)
	}
	for _, want := range []string{"term → definition", "external anchor", "~20", "kernel-reducibility"} {
		if !strings.Contains(found.Summary, want) {
			t.Errorf("shared-codebook Summary missing %q", want)
		}
	}
}

func TestNew_PathGetters(t *testing.T) {
	c := New("/base")
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"SettingsSeed", c.SettingsSeed(), "/base/settings.json"},
		{"HudConfigSeed", c.HudConfigSeed(), "/base/claude-hud.json"},
		{"RTKConfigSeed", c.RTKConfigSeed(), "/base/rtk-config.toml"},
		{"PluginsTxt", c.PluginsTxt(), "/base/plugins.txt"},
		{"SkillsTxt", c.SkillsTxt(), "/base/skills.txt"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestReadStackCatalog(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		noConfig bool
		want     []string
	}{
		{
			name:     "missing file returns nil",
			noConfig: true,
			want:     nil,
		},
		{
			name:    "filters comments and blank lines",
			content: "# comment\n\nrust\ngo\n# another\npython\n",
			want:    []string{"rust", "go", "python"},
		},
		{
			name:    "empty file returns nil",
			content: "",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.noConfig {
				if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config", "stacks.txt"), []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := ReadStackCatalog(dir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadStackCatalog = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteStacks(t *testing.T) {
	t.Run("creates .env with STACKS line", func(t *testing.T) {
		dir := t.TempDir()
		if err := WriteStacks(dir, "go,rust"); err != nil {
			t.Fatal(err)
		}
		v, ok := ReadStacks(dir)
		if !ok || v != "go,rust" {
			t.Errorf("ReadStacks = (%q, %v), want (go,rust, true)", v, ok)
		}
	})

	t.Run("preserves other keys", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PLUGINS_DISABLED=plugin-x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := WriteStacks(dir, "go"); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "PLUGINS_DISABLED=plugin-x") {
			t.Error("WriteStacks clobbered PLUGINS_DISABLED")
		}
		v, _ := ReadStacks(dir)
		if v != "go" {
			t.Errorf("STACKS = %q, want go", v)
		}
	})

	t.Run("overwrites existing STACKS line", func(t *testing.T) {
		dir := t.TempDir()
		if err := WriteStacks(dir, "old"); err != nil {
			t.Fatal(err)
		}
		if err := WriteStacks(dir, "new"); err != nil {
			t.Fatal(err)
		}
		v, _ := ReadStacks(dir)
		if v != "new" {
			t.Errorf("STACKS after overwrite = %q, want new", v)
		}
		count := strings.Count(string(func() []byte {
			d, _ := os.ReadFile(filepath.Join(dir, ".env"))
			return d
		}()), "STACKS=")
		if count != 1 {
			t.Errorf("STACKS appears %d times, want 1", count)
		}
	})
}

func TestReadStacks_MissingLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("OTHER=val\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, ok := ReadStacks(dir)
	if ok || v != "" {
		t.Errorf("ReadStacks with no STACKS line = (%q, %v), want (\"\", false)", v, ok)
	}
}

func TestReadStacks_NoEnvFile(t *testing.T) {
	v, ok := ReadStacks(t.TempDir())
	if ok || v != "" {
		t.Errorf("ReadStacks with no .env = (%q, %v), want (\"\", false)", v, ok)
	}
}

func TestReadPluginsDisabled_MissingLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("OTHER=val\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadPluginsDisabled(dir); got != nil {
		t.Errorf("ReadPluginsDisabled with no PLUGINS_DISABLED line = %v, want nil", got)
	}
}

func TestReadPluginsDisabled_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PLUGINS_DISABLED=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ReadPluginsDisabled(dir)
	if got != nil {
		t.Errorf("ReadPluginsDisabled empty value = %v, want nil", got)
	}
}

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

func TestReadSkillCatalog(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		noConfig bool
		want     []string
	}{
		{
			name:     "missing file returns nil",
			noConfig: true,
			want:     nil,
		},
		{
			name:    "filters comments and blank lines",
			content: "# comment\n\nowner/skill-a\nowner/skill-b\n",
			want:    []string{"owner/skill-a", "owner/skill-b"},
		},
		{
			name:    "empty file returns nil",
			content: "",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.noConfig {
				if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config", "skills.txt"), []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := ReadSkillCatalog(dir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadSkillCatalog = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadSkills_NoEnvFile(t *testing.T) {
	v, ok := ReadSkills(t.TempDir())
	if ok || v != "" {
		t.Errorf("ReadSkills with no .env = (%q, %v), want (\"\", false)", v, ok)
	}
}

func TestReadSkills_MissingLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("OTHER=val\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, ok := ReadSkills(dir)
	if ok || v != "" {
		t.Errorf("ReadSkills with no SKILLS line = (%q, %v), want (\"\", false)", v, ok)
	}
}

func TestWriteSkills(t *testing.T) {
	t.Run("creates .env with SKILLS line", func(t *testing.T) {
		dir := t.TempDir()
		if err := WriteSkills(dir, "owner/skill-a,owner/skill-b"); err != nil {
			t.Fatal(err)
		}
		v, ok := ReadSkills(dir)
		if !ok || v != "owner/skill-a,owner/skill-b" {
			t.Errorf("ReadSkills = (%q, %v), want (owner/skill-a,owner/skill-b, true)", v, ok)
		}
	})

	t.Run("preserves other keys", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("STACKS=go\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := WriteSkills(dir, "owner/skill-a"); err != nil {
			t.Fatal(err)
		}
		stacks, _ := ReadStacks(dir)
		if stacks != "go" {
			t.Errorf("WriteSkills clobbered STACKS: got %q, want go", stacks)
		}
		v, _ := ReadSkills(dir)
		if v != "owner/skill-a" {
			t.Errorf("SKILLS = %q, want owner/skill-a", v)
		}
	})

	t.Run("overwrites existing SKILLS line", func(t *testing.T) {
		dir := t.TempDir()
		if err := WriteSkills(dir, "old"); err != nil {
			t.Fatal(err)
		}
		if err := WriteSkills(dir, "new"); err != nil {
			t.Fatal(err)
		}
		v, _ := ReadSkills(dir)
		if v != "new" {
			t.Errorf("SKILLS after overwrite = %q, want new", v)
		}
		count := strings.Count(string(func() []byte {
			d, _ := os.ReadFile(filepath.Join(dir, ".env"))
			return d
		}()), "SKILLS=")
		if count != 1 {
			t.Errorf("SKILLS appears %d times, want 1", count)
		}
	})
}
