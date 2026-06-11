package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestWarn_NilNoOutput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	warn("no-op step", nil)
	w.Close()
	os.Stderr = orig
	var buf [64]byte
	n, _ := r.Read(buf[:])
	r.Close()
	if n != 0 {
		t.Errorf("warn(nil) wrote %q to stderr, want empty", buf[:n])
	}
}

func TestWarn_ErrorWritesToStderr(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	warn("test-step", fmt.Errorf("something failed"))
	w.Close()
	os.Stderr = orig
	var buf [256]byte
	n, _ := r.Read(buf[:])
	r.Close()
	got := string(buf[:n])
	if !strings.Contains(got, "[provision] WARN: test-step: something failed") {
		t.Errorf("warn(err) stderr = %q, want to contain [provision] WARN: test-step: something failed", got)
	}
}

func TestHome_EnvSet(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := home()
	if got != tmp {
		t.Errorf("home() = %q, want %q", got, tmp)
	}
}

func TestHome_EnvUnset(t *testing.T) {
	t.Setenv("HOME", "")
	want, _ := os.UserHomeDir()
	if got := home(); got != want {
		t.Errorf("home() = %q, want %q (os.UserHomeDir fallback)", got, want)
	}
}

func TestReadHarnessChoice_NoFile_DefaultInstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := readHarnessChoice()
	if got != "install" {
		t.Errorf("readHarnessChoice() = %q, want install when file absent", got)
	}
}

func TestReadHarnessChoice_SkipFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-harness"), []byte("skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readHarnessChoice()
	if got != "skip" {
		t.Errorf("readHarnessChoice() = %q, want skip", got)
	}
}

func TestReadPluginCatalog_FileAbsent(t *testing.T) {
	cfg := config.New(t.TempDir())
	got := readPluginCatalog(cfg)
	if got != nil {
		t.Errorf("readPluginCatalog with no file = %v, want nil", got)
	}
}

func TestReadPluginCatalog_WithEntries(t *testing.T) {
	dir := t.TempDir()
	content := "plugin-a@v1\nplugin-b\n"
	if err := os.WriteFile(filepath.Join(dir, "plugins.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(dir)
	got := readPluginCatalog(cfg)
	if len(got) != 2 {
		t.Fatalf("readPluginCatalog() = %v, want 2 entries", got)
	}
	if got[0] != "plugin-a@v1" || got[1] != "plugin-b" {
		t.Errorf("readPluginCatalog() = %v, want [plugin-a@v1 plugin-b]", got)
	}
}

func TestReadDisabledPlugins_FileAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := readDisabledPlugins()
	if got != nil {
		t.Errorf("readDisabledPlugins with no file = %v, want nil", got)
	}
}

func TestReadDisabledPlugins_WithEntries(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "plugin-x\nplugin-y\n"
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-plugins-disabled"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readDisabledPlugins()
	if len(got) != 2 {
		t.Fatalf("readDisabledPlugins() = %v, want 2", got)
	}
	if got[0] != "plugin-x" || got[1] != "plugin-y" {
		t.Errorf("readDisabledPlugins() = %v, want [plugin-x plugin-y]", got)
	}
}

func TestToSlice_Nil(t *testing.T) {
	got := toSlice(nil)
	if got != nil {
		t.Errorf("toSlice(nil) = %v, want nil", got)
	}
}

func TestToSlice_Slice(t *testing.T) {
	in := []any{"a", "b"}
	got := toSlice(in)
	if len(got) != 2 {
		t.Fatalf("toSlice() = %v, want length 2", got)
	}
}

func TestToSlice_NonSlice(t *testing.T) {
	got := toSlice("not a slice")
	if got != nil {
		t.Errorf("toSlice(string) = %v, want nil", got)
	}
}

func TestRtkHookPresent_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if rtkHookPresent() {
		t.Error("rtkHookPresent = true when settings file absent")
	}
}

func TestRtkHookPresent_NoHooksKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := writeSettingsJSON(t, tmp, map[string]any{"theme": "dark"}); err != nil {
		t.Fatal(err)
	}
	if rtkHookPresent() {
		t.Error("rtkHookPresent = true when no hooks key")
	}
}

func TestRtkHookPresent_HookPresent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "rtk hook claude"},
					},
				},
			},
		},
	}
	if err := writeSettingsJSON(t, tmp, settings); err != nil {
		t.Fatal(err)
	}
	if !rtkHookPresent() {
		t.Error("rtkHookPresent = false when rtk hook claude is present")
	}
}

func TestRtkHookPresent_DifferentCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "some-other-hook"},
					},
				},
			},
		},
	}
	if err := writeSettingsJSON(t, tmp, settings); err != nil {
		t.Fatal(err)
	}
	if rtkHookPresent() {
		t.Error("rtkHookPresent = true when command does not match")
	}
}

func TestEnsureTheme_NoFile_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfg := config.New(tmp)
	if err := EnsureTheme(cfg); err != nil {
		t.Errorf("EnsureTheme with no theme file = %v, want nil", err)
	}
}

func TestEnsureTheme_EmptyFile_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-theme"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(tmp)
	if err := EnsureTheme(cfg); err != nil {
		t.Errorf("EnsureTheme with empty theme file = %v, want nil", err)
	}
}

func TestEnsureTheme_NoSettingsFile_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-theme"), []byte("dark\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(tmp)
	if err := EnsureTheme(cfg); err != nil {
		t.Errorf("EnsureTheme with no settings.json = %v, want nil", err)
	}
}

func TestEnsureTheme_WritesTheme(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-theme"), []byte("dark\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSettingsJSON(t, tmp, map[string]any{"theme": "light"}); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(tmp)
	if err := EnsureTheme(cfg); err != nil {
		t.Errorf("EnsureTheme = %v, want nil", err)
	}
	result, err := readJSON(filepath.Join(cd, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if result["theme"] != "dark" {
		t.Errorf("theme = %v, want dark", result["theme"])
	}
}

func TestHarnessInstalled_SkipPref(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-harness"), []byte("skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{}
	got, err := HarnessInstalled(context.Background(), r)
	if err != nil {
		t.Fatalf("HarnessInstalled: %v", err)
	}
	if !got {
		t.Error("HarnessInstalled = false when pref is skip, want true")
	}
}

func TestHarnessInstalled_PluginFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	got, err := HarnessInstalled(context.Background(), r)
	if err != nil {
		t.Fatalf("HarnessInstalled: %v", err)
	}
	if !got {
		t.Error("HarnessInstalled = false when container returns no error, want true")
	}
}

func TestHarnessInstalled_PluginMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("grep no match")
		},
	}
	got, err := HarnessInstalled(context.Background(), r)
	if err != nil {
		t.Fatalf("HarnessInstalled: %v", err)
	}
	if got {
		t.Error("HarnessInstalled = true when grep fails, want false")
	}
}

func TestRelinkHarness_Success(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	err := relinkHarness(context.Background(), r)
	if err != nil {
		t.Errorf("relinkHarness = %v, want nil", err)
	}
}

func TestRelinkHarness_Error(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("ln failed")
		},
	}
	err := relinkHarness(context.Background(), r)
	if err == nil {
		t.Error("relinkHarness must propagate container error")
	}
}

func TestRelinkHarness_ExportsPluginRoot(t *testing.T) {
	var got string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			got = strings.Join(args, " ")
			return "", nil
		},
	}
	if err := relinkHarness(context.Background(), r); err != nil {
		t.Fatalf("relinkHarness = %v, want nil", err)
	}
	if !strings.Contains(got, `export CLAUDE_PLUGIN_ROOT="$HOME/.neuro-matrix"`) {
		t.Errorf("relinkHarness command missing CLAUDE_PLUGIN_ROOT export; got %q", got)
	}
	if !strings.Contains(got, "grep -qxF") {
		t.Errorf("relinkHarness export must be idempotent via a grep guard; got %q", got)
	}
	if !strings.Contains(got, `>>"$HOME/.bashrc"`) {
		t.Errorf("relinkHarness must append the export to the shell profile; got %q", got)
	}
}

func harnessCmdKey(args []string) string { return strings.Join(args, " ") }

func TestEnsureHarness_ClaudeAbsent(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("command not found")
		},
	}
	if err := EnsureHarness(context.Background(), r); err != nil {
		t.Errorf("EnsureHarness claude absent = %v, want nil", err)
	}
}

func TestEnsureHarness_HappyPath(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := harnessCmdKey(args)
			called = append(called, k)
			return "", nil
		},
	}
	if err := EnsureHarness(context.Background(), r); err != nil {
		t.Errorf("EnsureHarness happy = %v, want nil", err)
	}
	wantContains := []string{
		"claude plugin marketplace add AlexShchuka/neuro-matrix",
		"claude plugin install neuro-matrix@neuro-matrix --scope user",
		"claude plugin update neuro-matrix@neuro-matrix",
	}
	for _, want := range wantContains {
		found := false
		for _, c := range called {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("EnsureHarness happy path: %q not called; got %v", want, called)
		}
	}
	relinkFound := false
	for _, c := range called {
		if strings.Contains(c, "ln -sfn") {
			relinkFound = true
		}
	}
	if !relinkFound {
		t.Errorf("EnsureHarness happy path: relinkHarness not called; got %v", called)
	}
}

func TestEnsureHarness_MarketplaceAddFails_UpdateSucceeds(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := harnessCmdKey(args)
			called = append(called, k)
			if k == "claude plugin marketplace add AlexShchuka/neuro-matrix" {
				return "", fmt.Errorf("add failed")
			}
			return "", nil
		},
	}
	if err := EnsureHarness(context.Background(), r); err != nil {
		t.Errorf("EnsureHarness marketplace-add-fails = %v, want nil", err)
	}
	updateFound := false
	for _, c := range called {
		if c == "claude plugin marketplace update neuro-matrix" {
			updateFound = true
		}
	}
	if !updateFound {
		t.Errorf("EnsureHarness: marketplace update not called after add failure; got %v", called)
	}
}

func TestEnsureHarness_MarketplaceAddAndUpdateBothFail(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := harnessCmdKey(args)
			called = append(called, k)
			if k == "claude plugin marketplace add AlexShchuka/neuro-matrix" {
				return "", fmt.Errorf("add failed")
			}
			if k == "claude plugin marketplace update neuro-matrix" {
				return "", fmt.Errorf("update failed")
			}
			return "", nil
		},
	}
	if err := EnsureHarness(context.Background(), r); err != nil {
		t.Errorf("EnsureHarness add+update fail = %v, want nil (warns and continues)", err)
	}
	installFound := false
	for _, c := range called {
		if strings.Contains(c, "plugin install") {
			installFound = true
		}
	}
	if !installFound {
		t.Errorf("EnsureHarness: install not called after add+update failure; got %v", called)
	}
}

func TestEnsureHarness_FinalVerifyFails_NoRelink(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := harnessCmdKey(args)
			called = append(called, k)
			if strings.Contains(k, "grep -q neuro-matrix") {
				return "", fmt.Errorf("not found")
			}
			return "", nil
		},
	}
	if err := EnsureHarness(context.Background(), r); err != nil {
		t.Errorf("EnsureHarness verify-fail = %v, want nil", err)
	}
	for _, c := range called {
		if strings.Contains(c, "ln -sfn") {
			t.Errorf("EnsureHarness: relinkHarness called after failed verify; got %v", called)
		}
	}
}

func TestEnsureHarness_InstallAndUpdateFail_WarnsContinues(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := harnessCmdKey(args)
			called = append(called, k)
			if k == "claude plugin install neuro-matrix@neuro-matrix --scope user" {
				return "", fmt.Errorf("install failed")
			}
			if k == "claude plugin update neuro-matrix@neuro-matrix" {
				return "", fmt.Errorf("update failed")
			}
			return "", nil
		},
	}
	if err := EnsureHarness(context.Background(), r); err != nil {
		t.Errorf("EnsureHarness install+update fail = %v, want nil", err)
	}
	relinkFound := false
	for _, c := range called {
		if strings.Contains(c, "ln -sfn") {
			relinkFound = true
		}
	}
	if !relinkFound {
		t.Errorf("EnsureHarness: relink not called after install+update failure; got %v", called)
	}
}

func TestReadProvisionStatus_OkReturnsEmpty(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "ok", nil
		},
	}
	got := readProvisionStatus(context.Background(), r)
	if got != "" {
		t.Errorf("readProvisionStatus(ok) = %q, want empty", got)
	}
}

func TestReadProvisionStatus_WarnedReturnsValue(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "3/12 warned", nil
		},
	}
	got := readProvisionStatus(context.Background(), r)
	if got != "3/12 warned" {
		t.Errorf("readProvisionStatus = %q, want 3/12 warned", got)
	}
}

func TestReadProvisionStatus_ErrorReturnsEmpty(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("container not running")
		},
	}
	got := readProvisionStatus(context.Background(), r)
	if got != "" {
		t.Errorf("readProvisionStatus on error = %q, want empty", got)
	}
}

func TestComputeStatus_ContainerDownNotStale(t *testing.T) {
	tmp := t.TempDir()
	r := &runner.FakeRunner{
		RepoVal: tmp,
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 3 && args[2] == "rev-list" {
				return "0", nil
			}
			if name == "docker" {
				return "false", nil
			}
			return "false", nil
		},
	}
	st := ComputeStatus(context.Background(), r)
	if st.CommitsBehind != 0 {
		t.Errorf("CommitsBehind = %d, want 0", st.CommitsBehind)
	}
	if st.ContainerUp {
		t.Error("ContainerUp = true, want false")
	}
}

func TestHarnessStatus_SkipPref(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
		ContFunc: func(args []string) (string, error) {
			return "skip", nil
		},
	}
	got := harnessStatus(context.Background(), r)
	if got != "off" {
		t.Errorf("harnessStatus = %q, want off when pref is skip", got)
	}
}

func TestHarnessStatus_ContainerDown(t *testing.T) {
	containerCalled := false
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "false", nil
		},
		ContFunc: func(args []string) (string, error) {
			containerCalled = true
			return "", nil
		},
	}
	got := harnessStatus(context.Background(), r)
	if got != "unknown" {
		t.Errorf("harnessStatus = %q, want unknown when container not running", got)
	}
	if containerCalled {
		t.Error("harnessStatus must not call Container when container is not running")
	}
}

func TestHarnessStatus_HarnessMissing(t *testing.T) {
	calls := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			calls++
			if calls == 1 {
				return "", nil
			}
			return "", fmt.Errorf("grep: not found")
		},
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
	}
	got := harnessStatus(context.Background(), r)
	if got != "missing" {
		t.Errorf("harnessStatus = %q, want missing when grep fails", got)
	}
}

func TestHarnessStatus_HarnessOn(t *testing.T) {
	calls := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			calls++
			return "", nil
		},
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
	}
	got := harnessStatus(context.Background(), r)
	if got != "on" {
		t.Errorf("harnessStatus = %q, want on when container up and neuro-matrix present", got)
	}
}

func TestEnsureGitIdentity_GhAuthFails(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "gh" && len(args) > 0 && args[0] == "auth" {
				return "", fmt.Errorf("not signed in")
			}
			return "", nil
		},
	}
	if err := EnsureGitIdentity(context.Background(), r); err != nil {
		t.Errorf("EnsureGitIdentity = %v, want nil", err)
	}
}

func TestEnsureGitIdentity_GitMissing(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "gh" {
				return "ok", nil
			}
			if name == "git" {
				return "", fmt.Errorf("not found")
			}
			return "", nil
		},
	}
	if err := EnsureGitIdentity(context.Background(), r); err != nil {
		t.Errorf("EnsureGitIdentity = %v, want nil", err)
	}
}

func TestEnsureGitIdentity_ValidUser(t *testing.T) {
	userJSON := `{"login":"testuser","name":"Test User","email":"test@example.com","id":"12345"}`
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "gh" && len(args) > 0 && args[0] == "api" {
				return userJSON, nil
			}
			return "", nil
		},
	}
	if err := EnsureGitIdentity(context.Background(), r); err != nil {
		t.Errorf("EnsureGitIdentity = %v, want nil", err)
	}
}

func TestEnsureGitIdentity_UserWithNoEmail(t *testing.T) {
	userJSON := `{"login":"testuser","name":"","email":"","id":"12345"}`
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "gh" && len(args) > 0 && args[0] == "api" {
				return userJSON, nil
			}
			return "", nil
		},
	}
	if err := EnsureGitIdentity(context.Background(), r); err != nil {
		t.Errorf("EnsureGitIdentity = %v, want nil", err)
	}
}

func makeSkillsConfig(t *testing.T, catalog string) config.Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skills.txt"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	return config.New(dir)
}

func writeSkillsStatefile(t *testing.T, homeDir, content string) {
	t.Helper()
	cd := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, FileSkills), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureSkills_CatalogEntryWithoutSlash_Skipped(t *testing.T) {
	// Covers the "len(parts) != 2 → continue" branch (line 74-75):
	// a catalog line that is not in owner/repo format is silently skipped.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeSkillsStatefile(t, tmp, "malformed-entry\n")
	var hostCalls []string
	var gitSubCmds []string
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			hostCalls = append(hostCalls, name)
			if name == "git" && len(args) > 0 {
				gitSubCmds = append(gitSubCmds, args[0])
			}
			return "", nil
		},
	}
	// Catalog has a malformed entry (no slash) and the selection matches it.
	cfg := makeSkillsConfig(t, "malformed-entry\n")
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil for malformed catalog entry", err)
	}
	// A malformed catalog entry (no slash) must never trigger a git clone/pull.
	_ = hostCalls // recorded for debugging; the assertion is on gitSubCmds
	for _, sub := range gitSubCmds {
		if sub == "clone" || sub == "pull" {
			t.Errorf("git %q was called for a malformed catalog entry; want no clone/pull", sub)
		}
	}
}

func TestEnsureSkills_NoCatalog_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			t.Errorf("unexpected host call: %s %v", name, args)
			return "", nil
		},
	}
	cfg := config.New(t.TempDir())
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil when catalog empty", err)
	}
}

func TestEnsureSkills_NothingSelected_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	hostCalled := false
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			hostCalled = true
			return "", nil
		},
	}
	cfg := makeSkillsConfig(t, "noamseg/interview-coach-skill\n")
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil when nothing selected", err)
	}
	if hostCalled {
		t.Error("EnsureSkills should not call host when nothing selected")
	}
}

func TestEnsureSkills_GitMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeSkillsStatefile(t, tmp, "noamseg/interview-coach-skill\n")
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "version" {
				return "", fmt.Errorf("not found")
			}
			return "", nil
		},
	}
	cfg := makeSkillsConfig(t, "noamseg/interview-coach-skill\n")
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil when git missing", err)
	}
}

func TestEnsureSkills_DeselectedEntry_Untouched(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeSkillsStatefile(t, tmp, "")
	var hostCalls []string
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			hostCalls = append(hostCalls, name+" "+strings.Join(args, " "))
			return "", nil
		},
	}
	cfg := makeSkillsConfig(t, "noamseg/interview-coach-skill\n")
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil", err)
	}
	for _, c := range hostCalls {
		if strings.Contains(c, "clone") || strings.Contains(c, "pull") {
			t.Errorf("EnsureSkills cloned/pulled deselected entry; got %v", hostCalls)
		}
	}
}

func TestEnsureSkills_ExistingRepoGitPull(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeSkillsStatefile(t, tmp, "noamseg/interview-coach-skill\n")
	skillDir := filepath.Join(tmp, ".claude", "skills", "interview-coach-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	pullCalled := false
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "-C" && len(args) >= 3 && args[2] == "pull" {
				pullCalled = true
				return "", nil
			}
			return "", nil
		},
	}
	cfg := makeSkillsConfig(t, "noamseg/interview-coach-skill\n")
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil", err)
	}
	if !pullCalled {
		t.Error("git pull not called for existing repo")
	}
}

func TestEnsureSkills_CloneNewRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeSkillsStatefile(t, tmp, "noamseg/interview-coach-skill\n")
	cloneCalled := false
	var cloneURL string
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "clone" {
				cloneCalled = true
				for _, a := range args {
					if strings.HasPrefix(a, "https://") {
						cloneURL = a
					}
				}
				return "", nil
			}
			return "", nil
		},
	}
	cfg := makeSkillsConfig(t, "noamseg/interview-coach-skill\n")
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil", err)
	}
	if !cloneCalled {
		t.Error("git clone not called for new repo")
	}
	want := "https://github.com/noamseg/interview-coach-skill.git"
	if cloneURL != want {
		t.Errorf("clone URL = %q, want %q", cloneURL, want)
	}
}

func TestEnsureSkills_MkdirFails_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeSkillsStatefile(t, tmp, "noamseg/interview-coach-skill\n")
	cd := filepath.Join(tmp, ".claude")
	if err := os.Chmod(cd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cd, 0o755) })
	if os.Getuid() == 0 {
		t.Skip("permission-based mkdir failure is not reproducible as root")
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
	}
	cfg := makeSkillsConfig(t, "noamseg/interview-coach-skill\n")
	if err := EnsureSkills(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureSkills = %v, want nil when mkdir fails", err)
	}
}

func TestEnsurePlugins_ClaudeAbsent_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("claude not found")
		},
	}
	cfg := config.New(t.TempDir())
	if err := EnsurePlugins(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsurePlugins with claude absent = %v, want nil", err)
	}
}

func TestEnsurePlugins_EmptyCatalog_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	callCount := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			callCount++
			return "", nil
		},
	}
	cfg := config.New(t.TempDir())
	if err := EnsurePlugins(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsurePlugins with empty catalog = %v, want nil", err)
	}
	if callCount != 1 {
		t.Errorf("EnsurePlugins empty catalog made %d container calls, want exactly 1 (claude check only)", callCount)
	}
}

func TestEnsurePlugins_WithCatalogInstalls(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeSettingsJSON(t, tmp, map[string]any{}); err != nil {
		t.Fatal(err)
	}

	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "plugins.txt"), []byte("my-plugin@v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	cfg := config.New(cfgDir)
	if err := EnsurePlugins(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsurePlugins = %v, want nil", err)
	}
}

func TestEnsureMCP_ClaudeAbsent(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("claude not found")
		},
	}
	if err := EnsureMCP(context.Background(), r); err != nil {
		t.Errorf("EnsureMCP with claude absent = %v, want nil", err)
	}
}

func TestEnsureMCP_UvxAbsent(t *testing.T) {
	calls := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			calls++
			if calls == 1 {
				return "", nil
			}
			if len(args) >= 3 && args[2] == "command -v uvx" {
				return "", fmt.Errorf("uvx not found")
			}
			return "", nil
		},
	}
	if err := EnsureMCP(context.Background(), r); err != nil {
		t.Errorf("EnsureMCP with uvx absent = %v, want nil", err)
	}
}

func TestEnsureMCP_UvxPresent(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	if err := EnsureMCP(context.Background(), r); err != nil {
		t.Errorf("EnsureMCP with uvx = %v, want nil", err)
	}
}

func TestEnsureRTK_RtkMissing(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "rtk" {
				return "", fmt.Errorf("command not found")
			}
			return "", nil
		},
	}
	cfg := config.New(t.TempDir())
	if err := EnsureRTK(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureRTK = %v, want nil when rtk missing", err)
	}
}

func TestEnsureRTK_HookAlreadyPresent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "rtk hook claude"},
					},
				},
			},
		},
	}
	if err := writeSettingsJSON(t, tmp, settings); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
	}
	cfg := config.New(t.TempDir())
	if err := EnsureRTK(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureRTK = %v, want nil when hook present", err)
	}
}

func TestEnsureRTK_HookNotPresent_RtkInit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := writeSettingsJSON(t, tmp, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	initCalled := false
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "timeout" {
				initCalled = true
				return "", nil
			}
			return "", nil
		},
	}
	cfg := config.New(t.TempDir())
	if err := EnsureRTK(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureRTK = %v, want nil", err)
	}
	if !initCalled {
		t.Error("rtk init was not called when hook absent")
	}
}

func TestWriteEnabledPlugins_NoSettingsFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := t.TempDir()
	cfg := config.New(cfgDir)
	err := writeEnabledPlugins(context.Background(), nil, cfg)
	if err != nil {
		t.Errorf("writeEnabledPlugins with no settings file = %v, want nil", err)
	}
}

func TestWriteEnabledPlugins_WithSettingsFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := writeSettingsJSON(t, tmp, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "plugins.txt"), []byte("plugin-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(cfgDir)
	err := writeEnabledPlugins(context.Background(), nil, cfg)
	if err != nil {
		t.Errorf("writeEnabledPlugins = %v, want nil", err)
	}
}

func TestCreate_RunsWithoutError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		RepoVal: tmp,
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("claude not available")
		},
	}
	cfg := config.New(t.TempDir())
	if err := Create(context.Background(), r, cfg); err != nil {
		t.Errorf("Create = %v, want nil", err)
	}
}

func TestStart_RunsWithoutError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		RepoVal: tmp,
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("claude not available")
		},
	}
	cfg := config.New(t.TempDir())
	if err := Start(context.Background(), r, cfg); err != nil {
		t.Errorf("Start = %v, want nil", err)
	}
}

func TestEnsureSettings_SeedAbsent_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfg := config.New(t.TempDir())
	if err := EnsureSettings(cfg); err != nil {
		t.Errorf("EnsureSettings seed absent = %v, want nil", err)
	}
}

func TestEnsureSettings_DestAbsent_CopiesSeed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := t.TempDir()
	seedContent := `{"key":"seedval"}` + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "settings.json"), []byte(seedContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(cfgDir)
	if err := EnsureSettings(cfg); err != nil {
		t.Fatalf("EnsureSettings dest absent = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("dest file not created: %v", err)
	}
	if !strings.Contains(string(got), "seedval") {
		t.Errorf("dest content = %q, want to contain seedval", string(got))
	}
}

func TestEnsureSettings_DestInvalidJSON_CopiesSeed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := t.TempDir()
	seedContent := `{"key":"from-seed"}` + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "settings.json"), []byte(seedContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(cfgDir)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, "settings.json"), []byte("not json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSettings(cfg); err != nil {
		t.Fatalf("EnsureSettings dest invalid JSON = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(cd, "settings.json"))
	if err != nil {
		t.Fatalf("dest file not readable: %v", err)
	}
	if !strings.Contains(string(got), "from-seed") {
		t.Errorf("dest content = %q, want to contain from-seed", string(got))
	}
}

func TestEnsureSettings_WriteJSONFails_CopyFileFallback(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based write failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := t.TempDir()
	seedContent := `{"key":"fallback-seed"}` + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "settings.json"), []byte(seedContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(cfgDir)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	destPath := filepath.Join(cd, "settings.json")
	if err := os.WriteFile(destPath, []byte(`{"key":"orig"}`+"\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSettings(cfg); err == nil {
		t.Error("EnsureSettings = nil, want error when dest is read-only and copyFile fallback is also blocked")
	}
	if err := os.Chmod(destPath, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "orig") {
		t.Errorf("dest content = %q, want unchanged orig content", string(got))
	}
}

func TestWriteProvisionStatus_Ok(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeProvisionStatus(0, 10)
	data, err := os.ReadFile(filepath.Join(tmp, ".claude", ".mirabilis-provision-status"))
	if err != nil {
		t.Fatalf("status file not written: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("status = %q, want ok", string(data))
	}
}

func TestWriteProvisionStatus_Warned(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeProvisionStatus(3, 12)
	data, err := os.ReadFile(filepath.Join(tmp, ".claude", ".mirabilis-provision-status"))
	if err != nil {
		t.Fatalf("status file not written: %v", err)
	}
	if string(data) != "3/12 warned" {
		t.Errorf("status = %q, want 3/12 warned", string(data))
	}
}

func TestEnsureAll_WritesStatusFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		RepoVal: tmp,
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("claude not available")
		},
	}
	cfg := config.New(t.TempDir())
	if err := Create(context.Background(), r, cfg); err != nil {
		t.Fatalf("Create = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".claude", ".mirabilis-provision-status")); err != nil {
		t.Errorf("status file not written after Create: %v", err)
	}
}

func TestEnsureAll_SummaryLineOnWarns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cd, 0o755) })

	if os.Getuid() == 0 {
		t.Skip("permission-based mkdir failure is not reproducible as root")
	}

	r := &runner.FakeRunner{
		RepoVal: tmp,
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("claude not available")
		},
	}
	cfg := config.New(t.TempDir())

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = pw
	Create(context.Background(), r, cfg) //nolint:errcheck
	pw.Close()
	os.Stderr = orig
	var buf [65536]byte
	n, _ := pr.Read(buf[:])
	pr.Close()
	got := string(buf[:n])

	if !strings.Contains(got, "steps warned") {
		t.Errorf("stderr = %q, want '[provision] N of M steps warned'", got)
	}
}

func TestParseMCPList(t *testing.T) {
	tests := []struct {
		give string
		want map[string]bool
	}{
		{"", map[string]bool{}},
		{"context7 (http)\nsequential-thinking (stdio)\n", map[string]bool{"context7": true, "sequential-thinking": true}},
		{"  context7  \n", map[string]bool{"context7": true}},
		{"no-space-entry\n", map[string]bool{"no-space-entry": true}},
		{"\n\n", map[string]bool{}},
	}
	for _, tt := range tests {
		got := parseMCPList(tt.give)
		for k := range tt.want {
			if !got[k] {
				t.Errorf("parseMCPList(%q) missing %q", tt.give, k)
			}
		}
		for k := range got {
			if !tt.want[k] {
				t.Errorf("parseMCPList(%q) unexpected %q", tt.give, k)
			}
		}
	}
}

func TestEnsureMCP_AddIfAbsent_SkipsRegistered(t *testing.T) {
	var addedNames []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.Contains(joined, "command -v claude"):
				return "", nil
			case strings.Contains(joined, "command -v uvx"):
				return "", fmt.Errorf("no uvx")
			case joined == "claude mcp list":
				return "context7 (http)\n", nil
			case strings.Contains(joined, "mcp add") && strings.Contains(joined, "sequential-thinking"):
				addedNames = append(addedNames, "sequential-thinking")
				return "", nil
			case strings.Contains(joined, "mcp add") && strings.Contains(joined, "context7"):
				addedNames = append(addedNames, "context7")
				return "", nil
			}
			return "", nil
		},
	}
	if err := EnsureMCP(context.Background(), r); err != nil {
		t.Fatalf("EnsureMCP = %v, want nil", err)
	}
	for _, n := range addedNames {
		if n == "context7" {
			t.Error("context7 was added despite being already registered")
		}
	}
	found := false
	for _, n := range addedNames {
		if n == "sequential-thinking" {
			found = true
		}
	}
	if !found {
		t.Error("sequential-thinking not added despite being absent")
	}
}

func TestWriteProvisionStatus_MkdirFails_Noop(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	tmp := t.TempDir()
	ro := filepath.Join(tmp, "ro")
	if err := os.MkdirAll(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	t.Setenv("HOME", filepath.Join(ro, "home"))
	writeProvisionStatus(0, 10)
}

func TestReadSkillCatalog_SkipsBlankAndCommentLines(t *testing.T) {
	dir := t.TempDir()
	content := "# header\n\nowner/repo-a\n# another comment\nowner/repo-b\n"
	if err := os.WriteFile(filepath.Join(dir, "skills.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(dir)
	got := readSkillCatalog(cfg)
	if len(got) != 2 {
		t.Fatalf("readSkillCatalog = %v, want 2 entries (blank/comment lines skipped)", got)
	}
	if got[0] != "owner/repo-a" || got[1] != "owner/repo-b" {
		t.Errorf("readSkillCatalog = %v, want [owner/repo-a owner/repo-b]", got)
	}
}

func writeSettingsJSON(t *testing.T, homeDir string, m map[string]any) error {
	t.Helper()
	cd := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cd, "settings.json"), append(data, '\n'), 0o644)
}
