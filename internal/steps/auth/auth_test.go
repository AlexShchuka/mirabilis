package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestSteps_NameAndDeps(t *testing.T) {
	registered := Steps()
	if len(registered) != 1 {
		t.Fatalf("Steps() returned %d steps, want 1", len(registered))
	}
	s := registered[0]
	if s.Meta.Name != "gh" {
		t.Errorf("step Name = %q, want gh", s.Meta.Name)
	}
	found := false
	for _, d := range s.Meta.Deps {
		if d == "prepare" {
			found = true
		}
	}
	if !found {
		t.Errorf("step Deps = %v, want to include prepare", s.Meta.Deps)
	}
}

func TestCheck_ContainerOK(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "ok", nil
		},
	}
	got, err := step{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got {
		t.Error("Check = false, want true when container returns no error")
	}
}

func TestCheck_ContainerErrors(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("not signed in")
		},
	}
	got, err := step{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got {
		t.Error("Check = true, want false when container errors")
	}
}

func TestRun_ReturnsNil(t *testing.T) {
	r := &runner.FakeRunner{}
	err := step{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil", err)
	}
}
