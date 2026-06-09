package harness

import (
	"context"
	"fmt"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestSteps_NameAndDeps(t *testing.T) {
	registered := Steps()
	if len(registered) != 1 {
		t.Fatalf("Steps() returned %d, want 1", len(registered))
	}
	s := registered[0]
	if s.Meta.Name != "harness" {
		t.Errorf("Name = %q, want harness", s.Meta.Name)
	}
	found := false
	for _, d := range s.Meta.Deps {
		if d == "prepare" {
			found = true
		}
	}
	if !found {
		t.Errorf("Deps = %v, want to include prepare", s.Meta.Deps)
	}
}

func TestCheck_HarnessInstalled(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	got, err := step{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got {
		t.Error("Check = false when harness plugin list succeeds, want true")
	}
}

func TestCheck_HarnessNotInstalled(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("grep: no match")
		},
	}
	got, err := step{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got {
		t.Error("Check = true when grep fails, want false")
	}
}

func TestRun_WhenClaudeAbsent_ReturnsNil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("command not found")
		},
	}
	err := step{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil when claude absent", err)
	}
}
