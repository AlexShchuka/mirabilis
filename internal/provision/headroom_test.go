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
	if len(called) < 3 {
		t.Fatalf("EnsureHeadroom install: expected at least 3 calls, got %d: %v", len(called), called)
	}
	if !strings.Contains(called[0], "headroom mcp status") {
		t.Errorf("EnsureHeadroom install: call[0] = %q, want idempotency check", called[0])
	}
	if !strings.Contains(called[1], "headroom-ai[all]") {
		t.Errorf("EnsureHeadroom install: call[1] = %q, want venv+pip install", called[1])
	}
	if !strings.Contains(called[2], "headroom mcp install") {
		t.Errorf("EnsureHeadroom install: call[2] = %q, want headroom mcp install", called[2])
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
