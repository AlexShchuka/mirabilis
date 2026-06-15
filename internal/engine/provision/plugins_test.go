package provision

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func pluginsDeps(t *testing.T) (Deps, *exec.Fake) {
	t.Helper()
	d, f := testDeps(t)
	mustWrite(t, filepath.Join(d.Repo, "config", "marketplaces.txt"), "m1\n")
	return d, f
}

func enabledPlugins(t *testing.T, d Deps) map[string]any {
	t.Helper()
	m := mustReadJSON(t, d.settingsPath())
	enabled, _ := m["enabledPlugins"].(map[string]any)
	return enabled
}

func TestPluginsCheckTrueWhenListedAndEnabledMatch(t *testing.T) {
	t.Parallel()
	d, f := pluginsDeps(t)
	mustWrite(t, d.Cfg.PluginsTxt(), "alpha@1.0\n")
	mustWriteJSON(t, d.settingsPath(), map[string]any{
		"enabledPlugins": map[string]any{"neuro-matrix@neuro-matrix": true, "alpha@1.0": true},
	})
	f.Expect(script("command -v claude"), "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "alpha 1.0 enabled", nil)
	if !checkStep(t, &pluginsStep{d: d}) {
		t.Fatal("Check false but plugin listed and enabledPlugins matches")
	}
}

func TestPluginsCheckFalseWhenNotListed(t *testing.T) {
	t.Parallel()
	d, f := pluginsDeps(t)
	mustWrite(t, d.Cfg.PluginsTxt(), "alpha@1.0\n")
	mustWriteJSON(t, d.settingsPath(), map[string]any{})
	f.Expect(script("command -v claude"), "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "", nil)
	if checkStep(t, &pluginsStep{d: d}) {
		t.Fatal("Check true but alpha not in plugin list")
	}
}

func TestPluginsRunInstallsMissingSkipsDisabledWritesEnabled(t *testing.T) {
	t.Parallel()
	d, f := pluginsDeps(t)
	mustWrite(t, d.Cfg.PluginsTxt(), "alpha@1.0\nbeta\n")
	mustWrite(t, filepath.Join(d.claudeDir(), filePluginsDisabled), "beta\n")
	mustWriteJSON(t, d.settingsPath(), map[string]any{})
	f.Expect(script("command -v claude"), "", nil)
	f.Expect(script(`mkdir -p "$HOME/.cache/tmp"`), "", nil)
	f.Expect([]string{"claude", "plugin", "marketplace", "add", "m1"}, "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "", nil)
	f.Expect(script(`TMPDIR="$HOME/.cache/tmp" claude plugin install "alpha@1.0" --scope user`), "", nil)
	if err := runStep(t, &pluginsStep{d: d}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("unused stubs: %d (disabled plugin must not install)", n)
	}
	want := map[string]any{"neuro-matrix@neuro-matrix": true, "alpha@1.0": true}
	if got := enabledPlugins(t, d); !reflect.DeepEqual(got, want) {
		t.Errorf("enabledPlugins = %#v, want %#v (beta disabled, excluded)", got, want)
	}
}

func TestPluginsRunSkipsAlreadyListed(t *testing.T) {
	t.Parallel()
	d, f := pluginsDeps(t)
	mustWrite(t, d.Cfg.PluginsTxt(), "alpha@1.0\n")
	mustWriteJSON(t, d.settingsPath(), map[string]any{})
	f.Expect(script("command -v claude"), "", nil)
	f.Expect(script(`mkdir -p "$HOME/.cache/tmp"`), "", nil)
	f.Expect([]string{"claude", "plugin", "marketplace", "add", "m1"}, "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "alpha 1.0 enabled", nil)
	if err := runStep(t, &pluginsStep{d: d}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("install ran for already-listed plugin; unused stubs: %d", n)
	}
}

func TestPluginsHarnessSkipExcludesNeuroMatrix(t *testing.T) {
	t.Parallel()
	d, f := pluginsDeps(t)
	mustWrite(t, d.Cfg.PluginsTxt(), "alpha@1.0\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileHarness), harnessSkip+"\n")
	mustWriteJSON(t, d.settingsPath(), map[string]any{
		"enabledPlugins": map[string]any{"alpha@1.0": true},
	})
	f.Expect(script("command -v claude"), "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "alpha 1.0 enabled", nil)
	if !checkStep(t, &pluginsStep{d: d}) {
		t.Fatal("Check false but harness skipped → neuro-matrix must be excluded from enabledPlugins")
	}
}

func TestPluginsGracefulWhenClaudeAbsent(t *testing.T) {
	t.Parallel()
	d, f := pluginsDeps(t)
	mustWrite(t, d.Cfg.PluginsTxt(), "alpha@1.0\n")
	f.Expect(script("command -v claude"), "", errStub)
	f.Expect(script("command -v claude"), "", errStub)
	s := &pluginsStep{d: d}
	if !checkStep(t, s) {
		t.Fatal("Check should degrade to true when claude absent")
	}
	if err := runStep(t, s); err != nil {
		t.Fatalf("Run should noop when claude absent: %v", err)
	}
}
