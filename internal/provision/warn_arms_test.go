package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func captureStderr(t *testing.T) (restore func() string) {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = pw
	t.Cleanup(func() {
		pw.Close()
		os.Stderr = old
	})
	return func() string {
		pw.Close()
		os.Stderr = old
		var buf [65536]byte
		n, _ := pr.Read(buf[:])
		pr.Close()
		return string(buf[:n])
	}
}

func TestEnsureGitIdentity_GitConfigNameWarns_Continues(t *testing.T) {
	userJSON := `{"login":"u","name":"N","email":"e@x.io","id":"1"}`
	emailSet := false
	setupGit := false
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			switch {
			case name == "gh" && len(args) >= 1 && args[0] == "api":
				return userJSON, nil
			case name == "gh" && len(args) >= 2 && args[1] == "setup-git":
				setupGit = true
				return "", nil
			case name == "gh":
				return "ok", nil
			case name == "git" && len(args) >= 1 && args[0] == "version":
				return "ok", nil
			case name == "git" && len(args) >= 3 && args[2] == "user.name":
				return "", fmt.Errorf("config name boom")
			case name == "git" && len(args) >= 3 && args[2] == "user.email":
				emailSet = true
				return "", nil
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsureGitIdentity(context.Background(), r)
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureGitIdentity = %v, want nil despite git config name failure", err)
	}
	if !strings.Contains(out, "git config user.name") {
		t.Errorf("stderr = %q, want warning naming git config user.name", out)
	}
	if !emailSet || !setupGit {
		t.Errorf("steps after the failing one did not run (emailSet=%v setupGit=%v)", emailSet, setupGit)
	}
}

func TestEnsureGitIdentity_SetupGitWarns_Continues(t *testing.T) {
	userJSON := `{"login":"u","name":"N","email":"e@x.io","id":"1"}`
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			switch {
			case name == "gh" && len(args) >= 1 && args[0] == "api":
				return userJSON, nil
			case name == "gh" && len(args) >= 2 && args[1] == "setup-git":
				return "", fmt.Errorf("setup-git boom")
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsureGitIdentity(context.Background(), r)
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureGitIdentity = %v, want nil despite setup-git failure", err)
	}
	if !strings.Contains(out, "gh auth setup-git") {
		t.Errorf("stderr = %q, want warning naming gh auth setup-git", out)
	}
}

func TestEnsureGitIdentity_BadUserJSON_Noop(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "gh" && len(args) >= 1 && args[0] == "api" {
				return "{not json", nil
			}
			return "", nil
		},
	}
	if err := EnsureGitIdentity(context.Background(), r); err != nil {
		t.Errorf("EnsureGitIdentity = %v, want nil when user JSON is undecodable", err)
	}
}

func TestEnsurePlugins_MarketplaceAddWarns_Continues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := writeSettingsJSON(t, tmp, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "plugins.txt"), []byte("p@v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	installed := false
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.Contains(joined, "marketplace add"):
				return "", fmt.Errorf("marketplace boom")
			case strings.Contains(joined, "plugin install"):
				installed = true
				return "", nil
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsurePlugins(context.Background(), r, config.New(cfgDir))
	out := getErr()
	if err != nil {
		t.Fatalf("EnsurePlugins = %v, want nil despite marketplace add failure", err)
	}
	if !strings.Contains(out, "marketplace add") {
		t.Errorf("stderr = %q, want WARN naming marketplace add", out)
	}
	if !installed {
		t.Error("plugin install did not run after marketplace add warned")
	}
}

func TestEnsurePlugins_InstallWarns_Continues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := writeSettingsJSON(t, tmp, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "plugins.txt"), []byte("p@v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			if strings.Contains(strings.Join(args, " "), "plugin install") {
				return "", fmt.Errorf("install boom")
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsurePlugins(context.Background(), r, config.New(cfgDir))
	out := getErr()
	if err != nil {
		t.Fatalf("EnsurePlugins = %v, want nil despite plugin install failure", err)
	}
	if !strings.Contains(out, "plugin install p@v1") {
		t.Errorf("stderr = %q, want WARN naming plugin install p@v1", out)
	}
	if strings.Contains(out, "marketplace add") {
		t.Errorf("stderr = %q, want no marketplace add WARN when it succeeded", out)
	}
}

func TestWriteEnabledPlugins_ReadJSONWarns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	getErr := captureStderr(t)
	err := writeEnabledPlugins(context.Background(), &runner.FakeRunner{}, config.New(t.TempDir()))
	out := getErr()
	if err != nil {
		t.Fatalf("writeEnabledPlugins = %v, want nil when settings unreadable", err)
	}
	if !strings.Contains(out, "read settings for enabledPlugins") {
		t.Errorf("stderr = %q, want WARN naming read settings for enabledPlugins", out)
	}
}

func TestWriteEnabledPlugins_WriteWarns(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based write failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(cd, "settings.json")
	if err := os.WriteFile(dest, []byte("{}\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	getErr := captureStderr(t)
	err := writeEnabledPlugins(context.Background(), &runner.FakeRunner{}, config.New(t.TempDir()))
	out := getErr()
	if err := os.Chmod(dest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("writeEnabledPlugins = %v, want nil even when write blocked", err)
	}
	if !strings.Contains(out, "write enabledPlugins") {
		t.Errorf("stderr = %q, want WARN naming write enabledPlugins", out)
	}
}

func TestEnsureTheme_ReadJSONWarns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-theme"), []byte("dark\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	getErr := captureStderr(t)
	err := EnsureTheme(config.New(t.TempDir()))
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureTheme = %v, want nil when settings unreadable", err)
	}
	if !strings.Contains(out, "read settings for theme") {
		t.Errorf("stderr = %q, want WARN naming read settings for theme", out)
	}
}

func TestEnsureTheme_WriteWarns(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based write failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-theme"), []byte("dark\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(cd, "settings.json")
	if err := os.WriteFile(dest, []byte("{}\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	getErr := captureStderr(t)
	err := EnsureTheme(config.New(t.TempDir()))
	out := getErr()
	if err := os.Chmod(dest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatalf("EnsureTheme = %v, want nil even when write blocked", err)
	}
	if !strings.Contains(out, "write settings for theme") {
		t.Errorf("stderr = %q, want WARN naming write settings for theme", out)
	}
}

func TestEnsureTheme_WhitespaceOnly_Noop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-theme"), []byte("\n\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, "settings.json"), []byte(`{"theme":"keep"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureTheme(config.New(t.TempDir())); err != nil {
		t.Fatalf("EnsureTheme = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(cd, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"theme": "keep"`) && !strings.Contains(string(got), `"theme":"keep"`) {
		t.Errorf("settings = %q, want theme left untouched for whitespace-only file", string(got))
	}
}

func TestEnsureSettings_MkdirClaudeFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based mkdir failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmp, "ro", "home"))
	if err := os.MkdirAll(filepath.Join(tmp, "ro"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(tmp, "ro"), 0o755) })
	if err := EnsureSettings(config.New(t.TempDir())); err == nil {
		t.Error("EnsureSettings = nil, want error when ~/.claude cannot be created")
	}
}

func TestEnsureSkills_PullWarns_Continues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	icDir := filepath.Join(tmp, ".claude", "skills", "interview-coach")
	if err := os.MkdirAll(filepath.Join(icDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 3 && args[2] == "pull" {
				return "", fmt.Errorf("pull boom")
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsureSkills(context.Background(), r)
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureSkills = %v, want nil despite pull failure", err)
	}
	if !strings.Contains(out, "interview-coach pull") {
		t.Errorf("stderr = %q, want WARN naming interview-coach pull", out)
	}
}

func TestEnsureSkills_CloneWarns_Continues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 1 && args[0] == "clone" {
				return "", fmt.Errorf("clone boom")
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsureSkills(context.Background(), r)
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureSkills = %v, want nil despite clone failure", err)
	}
	if !strings.Contains(out, "interview-coach skill not installed") {
		t.Errorf("stderr = %q, want WARN naming interview-coach skill not installed", out)
	}
}

func TestEnsureRTK_InitWarns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := writeSettingsJSON(t, tmp, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "timeout" {
				return "", fmt.Errorf("init boom")
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsureRTK(context.Background(), r, config.New(t.TempDir()))
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureRTK = %v, want nil despite rtk init failure", err)
	}
	if !strings.Contains(out, "rtk init") {
		t.Errorf("stderr = %q, want WARN naming rtk init", out)
	}
}

func TestRtkHookPresent_MalformedHookShapes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				"not-a-map",
				map[string]any{
					"hooks": []any{
						"not-a-map",
						map[string]any{"command": "something-else"},
					},
				},
			},
		},
	}
	if err := writeSettingsJSON(t, tmp, settings); err != nil {
		t.Fatal(err)
	}
	if rtkHookPresent() {
		t.Error("rtkHookPresent = true, want false for malformed/non-matching hook entries")
	}
}

func TestEnsureMemory_WriteWarns(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based write failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	memDir := filepath.Join(tmp, ".claude", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(memDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(memDir, 0o755) })
	getErr := captureStderr(t)
	err := EnsureMemory()
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureMemory = %v, want nil despite write failures", err)
	}
	if !strings.Contains(out, "write memory") {
		t.Errorf("stderr = %q, want WARN naming write memory", out)
	}
}

func TestEnsureHudConfig_MkdirFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based mkdir failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "claude-hud.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(tmp, ".claude", "plugins")
	if err := os.MkdirAll(plugins, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(plugins, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(plugins, 0o755) })
	err := EnsureHudConfig(config.New(cfgDir))
	if err == nil {
		t.Error("EnsureHudConfig = nil, want error when the config dir cannot be created")
	}
	if !strings.Contains(err.Error(), "mkdir claude-hud config dir") {
		t.Errorf("err = %v, want mkdir claude-hud config dir", err)
	}
}

func TestEnsureMCP_RegisterFailureWarns(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.Contains(joined, "command -v uvx"):
				return "", fmt.Errorf("no uvx")
			case len(args) >= 3 && args[1] == "mcp" && args[2] == "add" && strings.Contains(joined, "sequential-thinking"):
				return "", fmt.Errorf("register boom")
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsureMCP(context.Background(), r)
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureMCP = %v, want nil despite register failures", err)
	}
	if !strings.Contains(out, "failed to register sequential-thinking") {
		t.Errorf("stderr = %q, want WARN naming failed to register sequential-thinking", out)
	}
	if !strings.Contains(out, "registered context7") {
		t.Errorf("stderr = %q, want success line naming registered context7", out)
	}
	if strings.Contains(out, "failed to register context7") {
		t.Errorf("stderr = %q, want no failure WARN for context7 which registered cleanly", out)
	}
}

func TestEnsureGitIdentity_GitConfigEmailWarns_Continues(t *testing.T) {
	userJSON := `{"login":"u","name":"N","email":"e@x.io","id":"1"}`
	setupGit := false
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			switch {
			case name == "gh" && len(args) >= 1 && args[0] == "api":
				return userJSON, nil
			case name == "gh" && len(args) >= 2 && args[1] == "setup-git":
				setupGit = true
				return "", nil
			case name == "git" && len(args) >= 3 && args[2] == "user.email":
				return "", fmt.Errorf("config email boom")
			}
			return "", nil
		},
	}
	getErr := captureStderr(t)
	err := EnsureGitIdentity(context.Background(), r)
	out := getErr()
	if err != nil {
		t.Fatalf("EnsureGitIdentity = %v, want nil despite git config email failure", err)
	}
	if !strings.Contains(out, "git config user.email") {
		t.Errorf("stderr = %q, want warning naming git config user.email", out)
	}
	if !setupGit {
		t.Error("setup-git did not run after git config email warned")
	}
}

func TestEnsurePlugins_DisabledAndListedSkipped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeSettingsJSON(t, tmp, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-plugins-disabled"), []byte("off-plugin@v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgDir := t.TempDir()
	catalog := "# header comment\n\noff-plugin@v1\nalready-there@v2\nfresh@v3\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "plugins.txt"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	var installed []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.Contains(joined, "plugin list"):
				return "already-there", nil
			case strings.Contains(joined, "plugin install"):
				installed = append(installed, joined)
				return "", nil
			}
			return "", nil
		},
	}
	if err := EnsurePlugins(context.Background(), r, config.New(cfgDir)); err != nil {
		t.Fatalf("EnsurePlugins = %v, want nil", err)
	}
	joinedInstalls := strings.Join(installed, "|")
	if strings.Contains(joinedInstalls, "off-plugin") {
		t.Error("disabled plugin was installed, want skipped")
	}
	if strings.Contains(joinedInstalls, "already-there") {
		t.Error("already-listed plugin was installed, want skipped")
	}
	if !strings.Contains(joinedInstalls, "fresh@v3") {
		t.Errorf("fresh plugin not installed; installs = %v", installed)
	}
}

func TestRtkHookPresent_NoPreToolUseKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	settings := map[string]any{"hooks": map[string]any{"PostToolUse": []any{}}}
	if err := writeSettingsJSON(t, tmp, settings); err != nil {
		t.Fatal(err)
	}
	if rtkHookPresent() {
		t.Error("rtkHookPresent = true, want false when PreToolUse key is absent")
	}
}

func TestEnsureRTKConfig_MkdirFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based mkdir failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "rtk-config.toml"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgHome := filepath.Join(tmp, ".config")
	if err := os.MkdirAll(cfgHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cfgHome, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cfgHome, 0o755) })
	err := EnsureRTKConfig(config.New(cfgDir))
	if err == nil {
		t.Error("EnsureRTKConfig = nil, want error when the rtk config dir cannot be created")
	}
}

func TestEnsureSettings_MkdirXDGDataFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based mkdir failure is not reproducible as root")
	}
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
	err := EnsureSettings(config.New(t.TempDir()))
	if err == nil {
		t.Error("EnsureSettings = nil, want error when ~/.claude/xdg-data cannot be created")
	}
}

func TestWriteJSON_MarshalError(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "out.json")
	err := writeJSON(dest, map[string]any{"bad": make(chan int)})
	if err == nil {
		t.Error("writeJSON = nil, want error for an unmarshalable value")
	}
}

func TestCopyFile_SourceMissing(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst")
	err := copyFile(filepath.Join(t.TempDir(), "nope"), dst)
	if err == nil {
		t.Error("copyFile = nil, want error when the source does not exist")
	}
}
