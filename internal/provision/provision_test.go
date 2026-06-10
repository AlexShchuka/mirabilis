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
		"claude plugin update neuro-matrix",
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
			if k == "claude plugin update neuro-matrix" {
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
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
		HostFunc: func(name string, args []string) (string, error) {
			return "false", nil
		},
	}
	got := harnessStatus(context.Background(), r)
	if got != "unknown" {
		t.Errorf("harnessStatus = %q, want unknown when container not running", got)
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

func TestEnsureAptPackages_NoFile(t *testing.T) {
	r := &runner.FakeRunner{}
	cfg := config.New(t.TempDir())
	err := EnsureAptPackages(context.Background(), r, cfg)
	if err != nil {
		t.Errorf("EnsureAptPackages with no file = %v, want nil", err)
	}
}

func TestEnsureAptPackages_AllInstalled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apt-packages.txt"), []byte("curl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "ok", nil
		},
	}
	cfg := config.New(dir)
	if err := EnsureAptPackages(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureAptPackages = %v, want nil", err)
	}
}

func TestEnsureAptPackages_MissingPackage_UpdateFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apt-packages.txt"), []byte("missing-pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "dpkg" {
				return "", fmt.Errorf("not installed")
			}
			if name == "sudo" && len(args) > 0 && args[0] == "apt-get" && args[1] == "update" {
				return "", fmt.Errorf("update failed")
			}
			return "", nil
		},
	}
	cfg := config.New(dir)
	if err := EnsureAptPackages(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureAptPackages = %v, want nil (warns-and-continues)", err)
	}
}

func TestEnsureAptPackages_MissingPackage_InstallFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apt-packages.txt"), []byte("missing-pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "dpkg" {
				return "", fmt.Errorf("not installed")
			}
			if name == "sudo" && len(args) > 0 && args[0] == "apt-get" && args[1] == "install" {
				return "", fmt.Errorf("install failed")
			}
			return "", nil
		},
	}
	cfg := config.New(dir)
	if err := EnsureAptPackages(context.Background(), r, cfg); err != nil {
		t.Errorf("EnsureAptPackages = %v, want nil", err)
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

func TestEnsureSkills_GitMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "version" {
				return "", fmt.Errorf("not found")
			}
			return "", nil
		},
	}
	if err := EnsureSkills(context.Background(), r); err != nil {
		t.Errorf("EnsureSkills = %v, want nil when git missing", err)
	}
}

func TestEnsureSkills_MkdirFails_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.WriteFile(cd, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
	}
	if err := EnsureSkills(context.Background(), r); err != nil {
		t.Errorf("EnsureSkills = %v, want nil when mkdir fails", err)
	}
}

func TestEnsureSkills_ExistingRepoGitPull(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	icDir := filepath.Join(tmp, ".claude", "skills", "interview-coach")
	if err := os.MkdirAll(filepath.Join(icDir, ".git"), 0o755); err != nil {
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
	if err := EnsureSkills(context.Background(), r); err != nil {
		t.Errorf("EnsureSkills = %v, want nil", err)
	}
	if !pullCalled {
		t.Error("git pull not called for existing repo")
	}
}

func TestEnsureSkills_CloneNewRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cloneCalled := false
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "clone" {
				cloneCalled = true
				return "", nil
			}
			return "", nil
		},
	}
	if err := EnsureSkills(context.Background(), r); err != nil {
		t.Errorf("EnsureSkills = %v, want nil", err)
	}
	if !cloneCalled {
		t.Error("git clone not called for new repo")
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
