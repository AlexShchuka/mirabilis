package provision

import (
	"context"
	"fmt"
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
