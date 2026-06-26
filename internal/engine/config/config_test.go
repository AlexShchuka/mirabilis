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
		{"ReadMarketplaces", "marketplaces.txt", ReadMarketplaces},
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

func TestReadMCPCatalog(t *testing.T) {
	t.Run("missing file returns nil nil", func(t *testing.T) {
		got, err := ReadMCPCatalog(t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("malformed json returns error", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "config", "mcp.json"), "{not json")
		got, err := ReadMCPCatalog(dir)
		if err == nil {
			t.Fatal("expected error for malformed mcp.json, got nil")
		}
		if got != nil {
			t.Errorf("got %v on error, want nil", got)
		}
	})
	t.Run("parses entries", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "config", "mcp.json"),
			`[{"name":"context7","transport":"http","url":"https://example/mcp"},`+
				`{"name":"st","transport":"stdio","args":["npx","-y","pkg"]}]`)
		got, err := ReadMCPCatalog(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []MCPEntry{
			{Name: "context7", Transport: "http", URL: "https://example/mcp"},
			{Name: "st", Transport: "stdio", Args: []string{"npx", "-y", "pkg"}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestHeadroomURLs(t *testing.T) {
	if got := HeadroomBaseURL(); got != "http://127.0.0.1:8787" {
		t.Errorf("HeadroomBaseURL = %q", got)
	}
	if got := HeadroomStatsURL(); got != "http://127.0.0.1:8787/stats" {
		t.Errorf("HeadroomStatsURL = %q", got)
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

func TestLastHarness(t *testing.T) {
	t.Run("no .env returns not ok", func(t *testing.T) {
		v, ok := ReadLastHarness(t.TempDir())
		if ok || v != "" {
			t.Errorf("ReadLastHarness with no .env = (%q, %v), want (\"\", false)", v, ok)
		}
	})

	t.Run("roundtrip and overwrite once", func(t *testing.T) {
		dir := t.TempDir()
		if err := WriteLastHarness(dir, "reinstall"); err != nil {
			t.Fatal(err)
		}
		v, ok := ReadLastHarness(dir)
		if !ok || v != "reinstall" {
			t.Errorf("ReadLastHarness = (%q, %v), want (reinstall, true)", v, ok)
		}
		if err := WriteLastHarness(dir, "on"); err != nil {
			t.Fatal(err)
		}
		if v, _ := ReadLastHarness(dir); v != "on" {
			t.Errorf("ReadLastHarness after overwrite = %q, want on", v)
		}
		if count := strings.Count(readEnv(t, dir), "LAST_HARNESS="); count != 1 {
			t.Errorf("LAST_HARNESS appears %d times, want 1", count)
		}
	})

	t.Run("preserves unrelated lines", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, ".env"), "STACKS=go\n")
		if err := WriteLastHarness(dir, "off"); err != nil {
			t.Fatal(err)
		}
		if s, _ := ReadStacks(dir); s != "go" {
			t.Errorf("STACKS clobbered: %q", s)
		}
	})
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

func TestHeadroomMode(t *testing.T) {
	t.Run("default is cache", func(t *testing.T) {
		t.Setenv("HEADROOM_MODE", "")
		if got := HeadroomMode(t.TempDir()); got != "cache" {
			t.Errorf("HeadroomMode default = %q, want cache", got)
		}
	})

	t.Run("env override", func(t *testing.T) {
		t.Setenv("HEADROOM_MODE", "token")
		if got := HeadroomMode(t.TempDir()); got != "token" {
			t.Errorf("HeadroomMode with HEADROOM_MODE=token = %q, want token", got)
		}
	})

	t.Run("dotenv override", func(t *testing.T) {
		t.Setenv("HEADROOM_MODE", "")
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, ".env"), "HEADROOM_MODE=token\n")
		if got := HeadroomMode(dir); got != "token" {
			t.Errorf("HeadroomMode with .env HEADROOM_MODE=token = %q, want token", got)
		}
	})

	t.Run("dotenv takes precedence over env", func(t *testing.T) {
		t.Setenv("HEADROOM_MODE", "cache")
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, ".env"), "HEADROOM_MODE=token\n")
		if got := HeadroomMode(dir); got != "token" {
			t.Errorf("HeadroomMode dotenv should take precedence, got %q, want token", got)
		}
	})

	unknownCases := []string{"$(rm -rf /)", "cache; rm", "token\nmalicious", "turbo", "", " "}
	for _, bad := range unknownCases {
		bad := bad
		t.Run("unknown value falls back to cache: "+bad, func(t *testing.T) {
			t.Setenv("HEADROOM_MODE", bad)
			if got := HeadroomMode(t.TempDir()); got != "cache" {
				t.Errorf("HeadroomMode(%q) = %q, want cache (allowlist fallback)", bad, got)
			}
		})
	}
}

func TestSkillGroupsFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.txt")
	mustWriteFile(t, path, "# comment\ngolang owner/repo skill-a skill-b\nbare\nnamed only/repo\n\n")
	got := SkillGroupsFrom(path)
	want := []SkillGroup{
		{Name: "golang", Repo: "owner/repo", Skills: []string{"skill-a", "skill-b"}},
		{Name: "bare"},
		{Name: "named", Repo: "only/repo"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SkillGroupsFrom = %#v, want %#v", got, want)
	}
}

func TestSkillGroupsFromMissing(t *testing.T) {
	if got := SkillGroupsFrom(filepath.Join(t.TempDir(), "nope.txt")); got != nil {
		t.Fatalf("missing file: got %#v, want nil", got)
	}
}

func TestReadLoadoutCatalog(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "forge.txt"), "effort xhigh\nbatch on\n")
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "spark.txt"), "effort medium\npacks core\n")
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "notes.md"), "ignored")

	got := ReadLoadoutCatalog(dir)
	want := map[string]bool{"forge": true, "spark": true}
	if len(got) != len(want) {
		t.Fatalf("catalog = %v, want keys %v", got, want)
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected loadout %q in catalog %v", name, got)
		}
	}
}

func TestReadLoadoutCatalogMissingDir(t *testing.T) {
	if got := ReadLoadoutCatalog(t.TempDir()); got != nil {
		t.Errorf("missing loadouts dir: got %v, want nil", got)
	}
}

func TestWriteLoadoutActivatesBatch(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "spark.txt"), "effort medium\npacks core\n")
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "forge.txt"), "effort xhigh\nbatch on\n")

	if err := WriteLoadout(dir, "spark"); err != nil {
		t.Fatalf("WriteLoadout: %v", err)
	}
	if LaunchBatched(dir) {
		t.Fatal("spark has no batch; LaunchBatched should be false")
	}

	if err := WriteLoadout(dir, "forge"); err != nil {
		t.Fatalf("WriteLoadout: %v", err)
	}
	if name, ok := ReadLoadout(dir); !ok || name != "forge" {
		t.Fatalf("ReadLoadout = %q,%v want forge", name, ok)
	}
	if !LaunchBatched(dir) {
		t.Error("after selecting forge (batch on), LaunchBatched should be true")
	}
}

func TestLaunchBatchedDefaultForgeIsBatched(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "forge.txt"), "effort xhigh\nbatch on\n")
	if !LaunchBatched(dir) {
		t.Error("default loadout forge has batch on; LaunchBatched should be true with no LOADOUT set")
	}
}

func TestLaunchBatchedNovaIsBatched(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "nova.txt"), "effort max\nbatch on\n")
	if err := WriteLoadout(dir, "nova"); err != nil {
		t.Fatalf("WriteLoadout: %v", err)
	}
	if !LaunchBatched(dir) {
		t.Error("nova has batch on; LaunchBatched should be true")
	}
}

func TestEffortOverrideRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if v, ok := ReadEffortOverride(dir); ok || v != "" {
		t.Errorf("ReadEffortOverride with no .env = (%q,%v), want (\"\",false)", v, ok)
	}
	if err := WriteEffortOverride(dir, "max"); err != nil {
		t.Fatalf("WriteEffortOverride: %v", err)
	}
	if v, ok := ReadEffortOverride(dir); !ok || v != "max" {
		t.Errorf("ReadEffortOverride = (%q,%v), want (max,true)", v, ok)
	}
}

func TestEffortOverrideEmptyValueIsNotSet(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".env"), "EFFORT=\n")
	if v, ok := ReadEffortOverride(dir); ok || v != "" {
		t.Errorf("empty EFFORT line = (%q,%v), want unset", v, ok)
	}
}

func TestClearEffortOverrideRemovesLineAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLoadout(dir, "forge"); err != nil {
		t.Fatal(err)
	}
	if err := WriteEffortOverride(dir, "high"); err != nil {
		t.Fatal(err)
	}
	if err := ClearEffortOverride(dir); err != nil {
		t.Fatalf("ClearEffortOverride: %v", err)
	}
	if v, ok := ReadEffortOverride(dir); ok || v != "" {
		t.Errorf("after clear, ReadEffortOverride = (%q,%v), want unset", v, ok)
	}
	if name, ok := ReadLoadout(dir); !ok || name != "forge" {
		t.Errorf("clear removed unrelated LOADOUT line: (%q,%v)", name, ok)
	}
	before := readEnv(t, dir)
	if err := ClearEffortOverride(dir); err != nil {
		t.Fatalf("ClearEffortOverride second call: %v", err)
	}
	if after := readEnv(t, dir); after != before {
		t.Errorf("clear on an absent key rewrote .env: %q -> %q", before, after)
	}
}

func TestClearEffortOverrideNoEnvFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := ClearEffortOverride(dir); err != nil {
		t.Fatalf("ClearEffortOverride with no .env: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".env")); !os.IsNotExist(err) {
		t.Errorf(".env created by clear on missing file: err=%v", err)
	}
}

func TestBatchOverrideRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if v, ok := ReadBatchOverride(dir); ok || v {
		t.Errorf("ReadBatchOverride with no .env = (%v,%v), want (false,false)", v, ok)
	}
	if err := WriteBatchOverride(dir, true); err != nil {
		t.Fatalf("WriteBatchOverride: %v", err)
	}
	if v, ok := ReadBatchOverride(dir); !ok || !v {
		t.Errorf("ReadBatchOverride = (%v,%v), want (true,true)", v, ok)
	}
	if err := WriteBatchOverride(dir, false); err != nil {
		t.Fatalf("WriteBatchOverride off: %v", err)
	}
	if v, ok := ReadBatchOverride(dir); !ok || v {
		t.Errorf("ReadBatchOverride after off = (%v,%v), want (false,true)", v, ok)
	}
}

func TestLaunchBatchedPrefersFleetOverride(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "spark.txt"), "effort medium\npacks core\n")
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "forge.txt"), "effort xhigh\nbatch on\n")
	if err := WriteLoadout(dir, "spark"); err != nil {
		t.Fatal(err)
	}
	if LaunchBatched(dir) {
		t.Fatal("spark is solo; LaunchBatched should be false before override")
	}
	if err := WriteBatchOverride(dir, true); err != nil {
		t.Fatal(err)
	}
	if !LaunchBatched(dir) {
		t.Error("fleet override on must make LaunchBatched true even for a solo loadout")
	}

	if err := WriteLoadout(dir, "forge"); err != nil {
		t.Fatal(err)
	}
	if err := WriteBatchOverride(dir, false); err != nil {
		t.Fatal(err)
	}
	if LaunchBatched(dir) {
		t.Error("fleet override off must make LaunchBatched false even for a batch loadout")
	}
	if err := ClearBatchOverride(dir); err != nil {
		t.Fatal(err)
	}
	if !LaunchBatched(dir) {
		t.Error("after clearing the override, forge (batch on) should drive LaunchBatched true again")
	}
}

func TestEffectiveEffortPrefersOverrideElseLoadout(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "spark.txt"), "effort medium\npacks core\n")
	if err := WriteLoadout(dir, "spark"); err != nil {
		t.Fatal(err)
	}
	if got := EffectiveEffort(dir); got != "medium" {
		t.Errorf("EffectiveEffort with no override = %q, want medium (loadout)", got)
	}
	if err := WriteEffortOverride(dir, "max"); err != nil {
		t.Fatal(err)
	}
	if got := EffectiveEffort(dir); got != "max" {
		t.Errorf("EffectiveEffort with override = %q, want max", got)
	}
}

func TestEffectiveBatchMirrorsLaunchBatched(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "spark.txt"), "effort medium\npacks core\n")
	if err := WriteLoadout(dir, "spark"); err != nil {
		t.Fatal(err)
	}
	if EffectiveBatch(dir) {
		t.Error("EffectiveBatch = true for solo loadout with no override, want false")
	}
	if err := WriteBatchOverride(dir, true); err != nil {
		t.Fatal(err)
	}
	if !EffectiveBatch(dir) {
		t.Error("EffectiveBatch = false after fleet override on, want true")
	}
}

func TestReadPackCatalog(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "packs.txt"),
		"core plugins caveman@caveman claude-hud@claude-hud\n"+
			"research mcp arxiv-mcp-server docling\n"+
			"formal tools sympy z3\n"+
			"bogus weird a b\n")
	packs := ReadPackCatalog(dir)
	if len(packs) != 3 {
		t.Fatalf("ReadPackCatalog = %d packs, want 3 (bogus kind dropped)", len(packs))
	}
	if got := packs["core"]; got.Kind != "plugins" || len(got.Items) != 2 {
		t.Errorf("core pack = %+v, want plugins with 2 items", got)
	}
	if got := packs["research"]; got.Kind != "mcp" || got.Items[0] != "arxiv-mcp-server" {
		t.Errorf("research pack = %+v, want mcp arxiv-mcp-server…", got)
	}
	if _, ok := packs["bogus"]; ok {
		t.Error("pack with unknown kind must be dropped")
	}
}

func TestLoadoutManifestExpandsPacks(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "packs.txt"),
		"core plugins caveman@caveman claude-hud@claude-hud\n"+
			"review plugins code-review@claude-plugins-official\n"+
			"research mcp arxiv-mcp-server docling\n"+
			"formal tools sympy z3\n")
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "forge.txt"),
		"effort xhigh\npacks core review formal\nbatch on\n")

	lo, ok := ReadLoadoutManifest(dir, "forge")
	if !ok {
		t.Fatal("ReadLoadoutManifest forge not ok")
	}
	wantPlugins := []string{"caveman@caveman", "claude-hud@claude-hud", "code-review@claude-plugins-official"}
	if !reflect.DeepEqual(lo.Plugins, wantPlugins) {
		t.Errorf("plugins = %v, want %v", lo.Plugins, wantPlugins)
	}
	if !reflect.DeepEqual(lo.Tools, []string{"sympy", "z3"}) {
		t.Errorf("tools = %v, want [sympy z3]", lo.Tools)
	}
	if len(lo.MCP) != 0 {
		t.Errorf("mcp = %v, want empty (research pack not referenced)", lo.MCP)
	}
	if !lo.Batch {
		t.Error("forge batch should be on")
	}
}

func TestLoadoutManifestPacksMergeWithExplicitAndDedupe(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "packs.txt"),
		"core plugins caveman@caveman claude-hud@claude-hud\n")
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "mix.txt"),
		"packs core\nplugins caveman@caveman extra@m\n")

	lo, ok := ReadLoadoutManifest(dir, "mix")
	if !ok {
		t.Fatal("ReadLoadoutManifest mix not ok")
	}
	want := []string{"caveman@caveman", "claude-hud@claude-hud", "extra@m"}
	if !reflect.DeepEqual(lo.Plugins, want) {
		t.Errorf("plugins = %v, want %v (pack+explicit merged, caveman deduped)", lo.Plugins, want)
	}
}

func TestLoadoutManifestUnknownPackIgnored(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "config", "packs.txt"),
		"core plugins caveman@caveman\n")
	mustWriteFile(t, filepath.Join(dir, "config", "loadouts", "x.txt"),
		"packs core nope\n")
	lo, ok := ReadLoadoutManifest(dir, "x")
	if !ok {
		t.Fatal("not ok")
	}
	if !reflect.DeepEqual(lo.Plugins, []string{"caveman@caveman"}) {
		t.Errorf("plugins = %v, want only caveman (unknown pack nope ignored)", lo.Plugins)
	}
}
