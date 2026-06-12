package steps

import (
	"context"
	"fmt"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestHarnessSteps_NameAndDeps(t *testing.T) {
	registered := harnessSteps()
	if len(registered) != 1 {
		t.Fatalf("harnessSteps() returned %d, want 1", len(registered))
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

func TestHarnessRun_WhenClaudeAbsent_ReturnsNil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("command not found")
		},
	}
	err := harnessStep{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil when claude absent", err)
	}
}
