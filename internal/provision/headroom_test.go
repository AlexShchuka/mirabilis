package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestEnsureHeadroom_InstalledAndRegistered_Skip(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			if strings.Contains(k, "claude mcp get headroom") {
				return "headroom: /home/node/.headroom-venv/bin/headroom mcp serve", nil
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom installed+registered = %v, want nil", err)
	}
	if len(called) != 2 {
		t.Fatalf("EnsureHeadroom installed+registered: expected 2 calls (binary check, registration check), got %d: %v", len(called), called)
	}
	if !strings.Contains(called[0], "test -x") || !strings.Contains(called[0], ".headroom-venv/bin/headroom") {
		t.Errorf("EnsureHeadroom: call[0] = %q, want venv binary check", called[0])
	}
	if !strings.Contains(called[1], "claude mcp get headroom") {
		t.Errorf("EnsureHeadroom: call[1] = %q, want registration check", called[1])
	}
}

func TestEnsureHeadroom_InstallSequence(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			if strings.Contains(k, "test -x") {
				return "", fmt.Errorf("not installed")
			}
			if strings.Contains(k, "claude mcp get headroom") {
				return "", fmt.Errorf("no such server")
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom install = %v, want nil", err)
	}
	if len(called) != 7 {
		t.Fatalf("EnsureHeadroom install: expected 7 calls (binary check, venv, pip, symlink, get, remove, add), got %d: %v", len(called), called)
	}
	if !strings.Contains(called[1], "python3 -m venv") {
		t.Errorf("EnsureHeadroom install: call[1] = %q, want venv create", called[1])
	}
	if !strings.Contains(called[2], "headroom-ai[mcp,proxy]") {
		t.Errorf("EnsureHeadroom install: call[2] = %q, want pip install", called[2])
	}
	if !strings.Contains(called[3], "ln -sf") {
		t.Errorf("EnsureHeadroom install: call[3] = %q, want symlink", called[3])
	}
	if !strings.Contains(called[5], "mcp remove headroom") {
		t.Errorf("EnsureHeadroom install: call[5] = %q, want mcp remove", called[5])
	}
	add := called[6]
	if !strings.Contains(add, "claude mcp add --scope user --transport stdio headroom") ||
		!strings.Contains(add, `"$HOME/.headroom-venv/bin/headroom" mcp serve`) {
		t.Errorf("EnsureHeadroom install: call[6] = %q, want absolute-path registration", add)
	}
}

func TestEnsureHeadroom_ReRegistersBareCommand(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			if strings.Contains(k, "claude mcp get headroom") {
				return "headroom: headroom mcp serve", nil
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom re-register = %v, want nil", err)
	}
	if len(called) != 4 {
		t.Fatalf("EnsureHeadroom re-register: expected 4 calls (binary check, get, remove, add), got %d: %v", len(called), called)
	}
	if !strings.Contains(called[3], `"$HOME/.headroom-venv/bin/headroom" mcp serve`) {
		t.Errorf("EnsureHeadroom re-register: call[3] = %q, want absolute-path registration replacing bare command", called[3])
	}
}

func TestEnsureHeadroom_VenvFails_WarnStops(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			if strings.Contains(k, "test -x") || strings.Contains(k, "python3 -m venv") {
				return "", fmt.Errorf("fail")
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom venv-fail = %v, want nil (warn-and-continue)", err)
	}
	if len(called) != 2 {
		t.Errorf("EnsureHeadroom venv-fail: expected stop after venv failure (2 calls), got %d: %v", len(called), called)
	}
}

func TestEnsureHeadroom_PipFails_WarnStops(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			if strings.Contains(k, "test -x") {
				return "", fmt.Errorf("not installed")
			}
			if strings.Contains(k, "headroom-ai[mcp,proxy]") {
				return "", fmt.Errorf("pip install failed")
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom pip-fail = %v, want nil (warn-and-continue)", err)
	}
	for _, c := range called {
		if strings.Contains(c, "claude mcp") {
			t.Errorf("EnsureHeadroom pip-fail: must not reach registration after pip failure; calls: %v", called)
		}
	}
}

func TestEnsureHeadroom_SymlinkFails_WarnStops(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			if strings.Contains(k, "test -x") {
				return "", fmt.Errorf("not installed")
			}
			if strings.Contains(k, "ln -sf") {
				return "", fmt.Errorf("symlink failed")
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom symlink-fail = %v, want nil (warn-and-continue)", err)
	}
	for _, c := range called {
		if strings.Contains(c, "claude mcp") {
			t.Errorf("EnsureHeadroom symlink-fail: must not reach registration after symlink failure; calls: %v", called)
		}
	}
}

func TestEnsureHeadroom_RegistrationFails_WarnContinues(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			if strings.Contains(k, "claude mcp get headroom") {
				return "", fmt.Errorf("no such server")
			}
			if strings.Contains(k, "claude mcp add") {
				return "", fmt.Errorf("registration failed")
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom registration-fail = %v, want nil (warn-and-continue)", err)
	}
	added := false
	for _, c := range called {
		if strings.Contains(c, "claude mcp add") {
			added = true
		}
	}
	if !added {
		t.Errorf("EnsureHeadroom registration-fail: claude mcp add not attempted; calls: %v", called)
	}
}

func setupProxySettingsHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(cd, "settings.json"), map[string]any{"theme": "dark"}); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func TestEnsureHeadroomProxy_AlreadyReachable_SetsBaseURL(t *testing.T) {
	tmp := setupProxySettingsHome(t)
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			return "", nil
		},
	}
	if err := EnsureHeadroomProxy(context.Background(), r); err != nil {
		t.Fatalf("EnsureHeadroomProxy = %v, want nil", err)
	}
	if len(called) != 1 {
		t.Fatalf("expected 1 call (probe), got %d: %v", len(called), called)
	}
	if !strings.Contains(called[0], "curl -fsS http://127.0.0.1:8787/stats") {
		t.Errorf("call[0] = %q, want curl probe", called[0])
	}
	m, err := readJSON(filepath.Join(tmp, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	env, _ := m["env"].(map[string]any)
	if env == nil || env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8787" {
		t.Errorf("ANTHROPIC_BASE_URL not set; env = %v", env)
	}
}

func TestEnsureHeadroomProxy_Down_StartsAndPollSucceeds_SetsBaseURL(t *testing.T) {
	tmp := setupProxySettingsHome(t)
	callIdx := 0
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			callIdx++
			switch callIdx {
			case 1:
				return "", fmt.Errorf("not reachable")
			case 2:
				return "", nil
			case 3:
				return "", nil
			}
			return "", nil
		},
	}
	if err := EnsureHeadroomProxy(context.Background(), r); err != nil {
		t.Fatalf("EnsureHeadroomProxy = %v, want nil", err)
	}
	if len(called) != 3 {
		t.Fatalf("expected 3 calls (probe, start, poll), got %d: %v", len(called), called)
	}
	if !strings.Contains(called[1], "setsid nohup") || !strings.Contains(called[1], "headroom") {
		t.Errorf("call[1] = %q, want setsid nohup start", called[1])
	}
	if !strings.Contains(called[2], "seq 1 60") {
		t.Errorf("call[2] = %q, want poll loop with seq 1 60", called[2])
	}
	m, err := readJSON(filepath.Join(tmp, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	env, _ := m["env"].(map[string]any)
	if env == nil || env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8787" {
		t.Errorf("ANTHROPIC_BASE_URL not set after successful start; env = %v", env)
	}
}

func TestEnsureHeadroomProxy_Down_PollFails_RemovesBaseURL(t *testing.T) {
	tmp := setupProxySettingsHome(t)
	cd := filepath.Join(tmp, ".claude")
	if err := writeJSON(filepath.Join(cd, "settings.json"), map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "http://127.0.0.1:8787"},
	}); err != nil {
		t.Fatal(err)
	}
	callIdx := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			callIdx++
			if callIdx <= 2 {
				return "", fmt.Errorf("not reachable")
			}
			return "", fmt.Errorf("poll timed out")
		},
	}
	if err := EnsureHeadroomProxy(context.Background(), r); err != nil {
		t.Fatalf("EnsureHeadroomProxy = %v, want nil", err)
	}
	m, err := readJSON(filepath.Join(cd, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	env, _ := m["env"].(map[string]any)
	if env != nil && env["ANTHROPIC_BASE_URL"] != nil {
		t.Errorf("ANTHROPIC_BASE_URL should be removed on poll failure; env = %v", env)
	}
}

func TestEnsureHeadroomProxy_SetBaseURL_Idempotent(t *testing.T) {
	tmp := setupProxySettingsHome(t)
	cd := filepath.Join(tmp, ".claude")
	sp := filepath.Join(cd, "settings.json")
	if err := writeJSON(sp, map[string]any{
		"env": map[string]any{"ANTHROPIC_BASE_URL": "http://127.0.0.1:8787"},
	}); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	if err := EnsureHeadroomProxy(context.Background(), r); err != nil {
		t.Fatalf("EnsureHeadroomProxy = %v, want nil", err)
	}
	m, err := readJSON(sp)
	if err != nil {
		t.Fatal(err)
	}
	env, _ := m["env"].(map[string]any)
	if env == nil || env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8787" {
		t.Errorf("ANTHROPIC_BASE_URL should remain set; env = %v", env)
	}
}

func TestEnsureHeadroomProxy_RemoveBaseURL_AbsentIsNoop(t *testing.T) {
	tmp := setupProxySettingsHome(t)
	cd := filepath.Join(tmp, ".claude")
	sp := filepath.Join(cd, "settings.json")
	if err := writeJSON(sp, map[string]any{"theme": "dark"}); err != nil {
		t.Fatal(err)
	}
	callIdx := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			callIdx++
			return "", fmt.Errorf("always fail")
		},
	}
	if err := EnsureHeadroomProxy(context.Background(), r); err != nil {
		t.Fatalf("EnsureHeadroomProxy = %v, want nil", err)
	}
	m, err := readJSON(sp)
	if err != nil {
		t.Fatal(err)
	}
	if m["theme"] != "dark" {
		t.Errorf("settings mutated unexpectedly: %v", m)
	}
}

func TestSetBaseURL_NoSettingsFile_Warns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	getErr := captureStderr(t)
	setBaseURL()
	errOut := getErr()
	if !strings.Contains(errOut, "headroom proxy read settings") {
		t.Errorf("stderr = %q, want WARN naming headroom proxy read settings", errOut)
	}
}
