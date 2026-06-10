package provision

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestEnsureHeadroom_AlreadyInstalled_Skip(t *testing.T) {
	var called []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			called = append(called, strings.Join(args, " "))
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom already installed = %v, want nil", err)
	}
	if len(called) != 1 {
		t.Errorf("EnsureHeadroom already installed: expected exactly 1 call (idempotency check), got %d: %v", len(called), called)
	}
	if !strings.Contains(called[0], "headroom mcp status") {
		t.Errorf("EnsureHeadroom already installed: first call = %q, want headroom mcp status check", called[0])
	}
}

func TestEnsureHeadroom_InstallSequence(t *testing.T) {
	var called []string
	callNum := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			callNum++
			if callNum == 1 {
				return "", fmt.Errorf("command not found")
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom install = %v, want nil", err)
	}
	if len(called) != 5 {
		t.Fatalf("EnsureHeadroom install: expected 5 calls (status, venv, pip, symlink, mcp install), got %d: %v", len(called), called)
	}
	if !strings.Contains(called[0], "headroom mcp status") {
		t.Errorf("EnsureHeadroom install: call[0] = %q, want idempotency check", called[0])
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
	if !strings.Contains(called[4], "headroom mcp install") {
		t.Errorf("EnsureHeadroom install: call[4] = %q, want headroom mcp install", called[4])
	}
}

func TestEnsureHeadroom_VenvFails_WarnStops(t *testing.T) {
	var called []string
	callNum := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			called = append(called, strings.Join(args, " "))
			callNum++
			if callNum == 1 || callNum == 2 {
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

func TestEnsureHeadroom_SymlinkFails_WarnStops(t *testing.T) {
	var called []string
	callNum := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			callNum++
			if callNum == 1 {
				return "", fmt.Errorf("status fail")
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
		if strings.Contains(c, "headroom mcp install") {
			t.Errorf("EnsureHeadroom symlink-fail: must not reach mcp install after symlink failure; calls: %v", called)
		}
	}
}

func TestEnsureHeadroom_PipFails_WarnStops(t *testing.T) {
	var called []string
	callNum := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			callNum++
			if callNum == 1 {
				return "", fmt.Errorf("status fail")
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
		if strings.Contains(c, "headroom mcp install") {
			t.Errorf("EnsureHeadroom pip-fail: must not reach mcp install after pip failure; calls: %v", called)
		}
	}
}

func TestEnsureHeadroom_InstallFails_WarnContinues(t *testing.T) {
	callNum := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			callNum++
			if callNum == 1 {
				return "", fmt.Errorf("command not found")
			}
			return "", fmt.Errorf("pip install failed")
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom install-fail = %v, want nil (warn-and-continue)", err)
	}
}

func TestEnsureHeadroom_RegistrationFails_WarnContinues(t *testing.T) {
	var called []string
	callNum := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			k := strings.Join(args, " ")
			called = append(called, k)
			callNum++
			if callNum == 1 {
				return "", fmt.Errorf("command not found")
			}
			if strings.Contains(k, "headroom mcp install") {
				return "", fmt.Errorf("registration failed")
			}
			return "", nil
		},
	}
	if err := EnsureHeadroom(context.Background(), r); err != nil {
		t.Errorf("EnsureHeadroom registration-fail = %v, want nil (warn-and-continue)", err)
	}
	registrationCalled := false
	for _, c := range called {
		if strings.Contains(c, "headroom mcp install") {
			registrationCalled = true
		}
	}
	if !registrationCalled {
		t.Errorf("EnsureHeadroom registration-fail: headroom mcp install not attempted; calls: %v", called)
	}
}
