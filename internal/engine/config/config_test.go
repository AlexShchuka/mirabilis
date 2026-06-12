package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readEnv(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

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

func TestCatalogReaders(t *testing.T) {
	readers := []struct {
		name string
		file string
		fn   func(string) []string
	}{
		{"ReadStackCatalog", "stacks.txt", ReadStackCatalog},
		{"ReadPluginCatalog", "plugins.txt", ReadPluginCatalog},
		{"ReadSkillCatalog", "skills.txt", ReadSkillCatalog},
	}
	cases := []struct {
		name    string
		content string
		noFile  bool
		want    []string
	}{
		{name: "missing file returns nil", noFile: true, want: nil},
		{
			name:    "filters comments and blank lines",
			content: "# comment\n\nitem-a\nitem-b\n# another\nitem-c\n",
			want:    []string{"item-a", "item-b", "item-c"},
		},
		{name: "empty file returns nil", content: "", want: nil},
		{name: "trims whitespace", content: "  item-a  \n\titem-b\t\n", want: []string{"item-a", "item-b"}},
	}
	for _, r := range readers {
		for _, tc := range cases {
			t.Run(r.name+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				if !tc.noFile {
					mustWriteFile(t, filepath.Join(dir, "config", r.file), tc.content)
				}
				if got := r.fn(dir); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("%s = %v, want %v", r.name, got, tc.want)
				}
			})
		}
	}
}

func TestEnvStringKeys(t *testing.T) {
	keys := []struct {
		key   string
		write func(string, string) error
		read  func(string) (string, bool)
	}{
		{"STACKS", WriteStacks, ReadStacks},
		{"SKILLS", WriteSkills, ReadSkills},
	}
	for _, k := range keys {
		t.Run(k.key+"/no .env returns not ok", func(t *testing.T) {
			v, ok := k.read(t.TempDir())
			if ok || v != "" {
				t.Errorf("read with no .env = (%q, %v), want (\"\", false)", v, ok)
			}
		})

		t.Run(k.key+"/missing line returns not ok", func(t *testing.T) {
			dir := t.TempDir()
			mustWriteFile(t, filepath.Join(dir, ".env"), "OTHER=val\n")
			v, ok := k.read(dir)
			if ok || v != "" {
				t.Errorf("read with no %s line = (%q, %v), want (\"\", false)", k.key, v, ok)
			}
		})

		t.Run(k.key+"/creates .env", func(t *testing.T) {
			dir := t.TempDir()
			if err := k.write(dir, "a,b"); err != nil {
				t.Fatal(err)
			}
			v, ok := k.read(dir)
			if !ok || v != "a,b" {
				t.Errorf("read = (%q, %v), want (a,b, true)", v, ok)
			}
		})

		t.Run(k.key+"/trims value", func(t *testing.T) {
			dir := t.TempDir()
			mustWriteFile(t, filepath.Join(dir, ".env"), k.key+"= a,b \n")
			v, ok := k.read(dir)
			if !ok || v != "a,b" {
				t.Errorf("read = (%q, %v), want (a,b, true)", v, ok)
			}
		})

		t.Run(k.key+"/overwrites existing line once", func(t *testing.T) {
			dir := t.TempDir()
			if err := k.write(dir, "old"); err != nil {
				t.Fatal(err)
			}
			if err := k.write(dir, "new"); err != nil {
				t.Fatal(err)
			}
			v, _ := k.read(dir)
			if v != "new" {
				t.Errorf("%s after overwrite = %q, want new", k.key, v)
			}
			if count := strings.Count(readEnv(t, dir), k.key+"="); count != 1 {
				t.Errorf("%s appears %d times, want 1", k.key, count)
			}
		})
	}
}

func TestEnvWrite_PreservesUnrelatedLines(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".env"), "FOO=bar\n\nBAZ=qux\n")
	if err := WriteStacks(dir, "go"); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkills(dir, "owner/skill"); err != nil {
		t.Fatal(err)
	}
	if err := WritePluginsDisabled(dir, []string{"plugin-a"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteSock(dir, true); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, dir)
	want := "FOO=bar\n\nBAZ=qux\nSTACKS=go\nSKILLS=owner/skill\nPLUGINS_DISABLED=plugin-a\nSOCK=1\n"
	if got != want {
		t.Errorf(".env after roundtrips = %q, want %q", got, want)
	}
}

func TestReadPluginsDisabled(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "no .env returns nil", content: "", want: nil},
		{name: "missing line returns nil", content: "OTHER=val\n", want: nil},
		{name: "empty value returns nil", content: "PLUGINS_DISABLED=\n", want: nil},
		{name: "single plugin", content: "PLUGINS_DISABLED=plugin-a\n", want: []string{"plugin-a"}},
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
				mustWriteFile(t, filepath.Join(dir, ".env"), tt.content)
			}
			if got := ReadPluginsDisabled(dir); !reflect.DeepEqual(got, tt.want) {
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
		want := []string{"plugin-a", "plugin-b"}
		if got := ReadPluginsDisabled(dir); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("preserves existing STACKS line", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, ".env"), "STACKS=go,rust\n")
		if err := WritePluginsDisabled(dir, []string{"plugin-x"}); err != nil {
			t.Fatal(err)
		}
		if stacks, _ := ReadStacks(dir); stacks != "go,rust" {
			t.Errorf("STACKS was clobbered: got %q, want %q", stacks, "go,rust")
		}
		if got := ReadPluginsDisabled(dir); !reflect.DeepEqual(got, []string{"plugin-x"}) {
			t.Errorf("PLUGINS_DISABLED = %v, want [plugin-x]", got)
		}
	})

	t.Run("overwrites existing PLUGINS_DISABLED line", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, ".env"), "PLUGINS_DISABLED=old\n")
		if err := WritePluginsDisabled(dir, []string{"new"}); err != nil {
			t.Fatal(err)
		}
		if got := ReadPluginsDisabled(dir); !reflect.DeepEqual(got, []string{"new"}) {
			t.Errorf("got %v, want [new]", got)
		}
	})

	t.Run("empty disabled list writes empty CSV", func(t *testing.T) {
		dir := t.TempDir()
		if err := WritePluginsDisabled(dir, nil); err != nil {
			t.Fatal(err)
		}
		if got := ReadPluginsDisabled(dir); got != nil {
			t.Errorf("got %v, want nil for empty disabled list", got)
		}
		if !strings.Contains(readEnv(t, dir), "PLUGINS_DISABLED=\n") {
			t.Error(".env missing empty PLUGINS_DISABLED line")
		}
	})
}

func TestAuthProxyPort(t *testing.T) {
	tests := []struct {
		name    string
		content string
		noEnv   bool
		want    int
	}{
		{name: "no .env returns default", noEnv: true, want: 8788},
		{name: "missing key returns default", content: "OTHER=val\n", want: 8788},
		{name: "override", content: "AUTH_PROXY_PORT=9999\n", want: 9999},
		{name: "garbage returns default", content: "AUTH_PROXY_PORT=banana\n", want: 8788},
		{name: "empty value returns default", content: "AUTH_PROXY_PORT=\n", want: 8788},
		{name: "trims value", content: "AUTH_PROXY_PORT= 9000 \n", want: 9000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.noEnv {
				mustWriteFile(t, filepath.Join(dir, ".env"), tt.content)
			}
			if got := AuthProxyPort(dir); got != tt.want {
				t.Errorf("AuthProxyPort = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSock(t *testing.T) {
	t.Run("absent defaults to false", func(t *testing.T) {
		if Sock(t.TempDir()) {
			t.Error("Sock with no .env = true, want false")
		}
	})

	t.Run("non-1 value is false", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, ".env"), "SOCK=yes\n")
		if Sock(dir) {
			t.Error("Sock(SOCK=yes) = true, want false")
		}
	})

	t.Run("roundtrip", func(t *testing.T) {
		dir := t.TempDir()
		if err := WriteSock(dir, true); err != nil {
			t.Fatal(err)
		}
		if !Sock(dir) {
			t.Error("Sock after WriteSock(true) = false, want true")
		}
		if err := WriteSock(dir, false); err != nil {
			t.Fatal(err)
		}
		if Sock(dir) {
			t.Error("Sock after WriteSock(false) = true, want false")
		}
		if count := strings.Count(readEnv(t, dir), "SOCK="); count != 1 {
			t.Errorf("SOCK appears %d times, want 1", count)
		}
	})
}

func TestLogPath(t *testing.T) {
	want := filepath.Join("/repo", ".mirabilis", "host.log")
	if got := LogPath("/repo"); got != want {
		t.Errorf("LogPath = %q, want %q", got, want)
	}
}
