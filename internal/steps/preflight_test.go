package steps

import (
	"context"
	"fmt"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestPreflightSteps_NameAndDeps(t *testing.T) {
	registered := preflightSteps()
	if len(registered) != 1 {
		t.Fatalf("preflightSteps() returned %d, want 1", len(registered))
	}
	s := registered[0]
	if s.Meta.Name != "preflight" {
		t.Errorf("Name = %q, want preflight", s.Meta.Name)
	}
	for _, want := range []string{"prepare", "harness", "gh"} {
		found := false
		for _, d := range s.Meta.Deps {
			if d == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Deps = %v, missing %q", s.Meta.Deps, want)
		}
	}
}

func TestPreflightCheck_AlwaysFalse(t *testing.T) {
	r := &runner.FakeRunner{}
	impl := preflightStep{}
	got, err := impl.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got {
		t.Error("Check = true, want false (preflight always runs)")
	}
}

func TestPreflightRun_Code200_Nil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "200", nil
		},
	}
	impl := preflightStep{}
	err := impl.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil for HTTP 200", err)
	}
}

func TestPreflightRun_Code401_Nil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "401", nil
		},
	}
	impl := preflightStep{}
	err := impl.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil for HTTP 401", err)
	}
}

func TestPreflightRun_Code403_Nil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "403", nil
		},
	}
	impl := preflightStep{}
	err := impl.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil for HTTP 403", err)
	}
}

func TestPreflightRun_EmptyCode_Unreachable(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	impl := preflightStep{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Fatal("Run must error when HTTP code is empty")
	}
}

func TestPreflightRun_Code000_Unreachable(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "000", nil
		},
	}
	impl := preflightStep{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Fatal("Run must error when HTTP code is 000")
	}
	if err.Error() == "" {
		t.Error("error message is empty")
	}
}

func TestPreflightRun_Code500_HTTPError(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "500", nil
		},
	}
	impl := preflightStep{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Fatal("Run must error for HTTP 500")
	}
	if err.Error() == "" {
		t.Error("error message is empty for HTTP 500")
	}
}

func TestPreflightRun_ContainerError_Unreachable(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", fmt.Errorf("network error")
		},
	}
	impl := preflightStep{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Error("Run must error when container call fails with empty code")
	}
}
