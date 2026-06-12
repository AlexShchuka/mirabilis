package steps

import (
	"context"
	"fmt"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestClaudeSteps_NamesAndDeps(t *testing.T) {
	registered := claudeSteps()
	if len(registered) != 2 {
		t.Fatalf("claudeSteps() returned %d steps, want 2", len(registered))
	}
	names := map[string]bool{}
	for _, r := range registered {
		names[r.Meta.Name] = true
	}
	for _, want := range []string{"claude", "theme"} {
		if !names[want] {
			t.Errorf("step %q not found in claudeSteps()", want)
		}
	}
	for _, r := range registered {
		found := false
		for _, d := range r.Meta.Deps {
			if d == "prepare" {
				found = true
			}
		}
		if !found {
			t.Errorf("step %q Deps = %v, want to include prepare", r.Meta.Name, r.Meta.Deps)
		}
	}
}

func TestClaudeRun_PropagatesContainerError(t *testing.T) {
	want := fmt.Errorf("container exec failed")
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", want
		},
	}
	impl := claudeStep{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Fatal("claudeStep.Run must propagate container error")
	}
}

func TestClaudeRun_SuccessReturnsNil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "ok", nil
		},
	}
	impl := claudeStep{}
	err := impl.Run(context.Background(), r)
	if err != nil {
		t.Errorf("claudeStep.Run = %v, want nil on success", err)
	}
}

func TestThemeCheck_EmptyOutputFalse(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	impl := themeStep{}
	got, err := impl.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("themeStep.Check: %v", err)
	}
	if got {
		t.Error("themeStep.Check = true, want false when output is empty")
	}
}

func TestThemeCheck_NonEmptyOutputTrue(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "dark", nil
		},
	}
	impl := themeStep{}
	got, err := impl.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("themeStep.Check: %v", err)
	}
	if !got {
		t.Error("themeStep.Check = false, want true when output is non-empty")
	}
}

func TestThemeRun_PropagatesError(t *testing.T) {
	want := fmt.Errorf("jq error")
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", want
		},
	}
	impl := themeStep{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Error("themeStep.Run must propagate container error")
	}
}

func TestThemeRun_SuccessReturnsNil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "ok", nil
		},
	}
	impl := themeStep{}
	err := impl.Run(context.Background(), r)
	if err != nil {
		t.Errorf("themeStep.Run = %v, want nil on success", err)
	}
}
