package steps_test

import (
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/steps"
	_ "github.com/AlexShchuka/mirabilis/internal/steps/auth"
	_ "github.com/AlexShchuka/mirabilis/internal/steps/claude"
	_ "github.com/AlexShchuka/mirabilis/internal/steps/container"
	_ "github.com/AlexShchuka/mirabilis/internal/steps/harness"
	_ "github.com/AlexShchuka/mirabilis/internal/steps/plugins"
	_ "github.com/AlexShchuka/mirabilis/internal/steps/preflight"
)

func TestRegistryShape(t *testing.T) {
	saved := steps.TakeSnapshot()
	defer steps.RestoreSnapshot(saved)

	registered := steps.BuildSteps()

	wantNames := []string{"update", "prepare", "claude", "theme", "harness", "plugins", "gh", "preflight"}
	if len(registered) != len(wantNames) {
		t.Fatalf("BuildSteps() returned %d steps, want %d", len(registered), len(wantNames))
	}

	byName := make(map[string]int, len(registered))
	for i, r := range registered {
		if _, dup := byName[r.Meta.Name]; dup {
			t.Errorf("duplicate step name %q in BuildSteps output", r.Meta.Name)
		}
		byName[r.Meta.Name] = i
	}

	for _, name := range wantNames {
		if _, ok := byName[name]; !ok {
			t.Errorf("expected step %q not found in BuildSteps output", name)
		}
	}

	for _, r := range registered {
		pos := byName[r.Meta.Name]
		for _, dep := range r.Meta.Deps {
			depPos, ok := byName[dep]
			if !ok {
				t.Errorf("step %q depends on unknown step %q", r.Meta.Name, dep)
				continue
			}
			if depPos >= pos {
				t.Errorf("step %q (pos %d) appears before its dep %q (pos %d)", r.Meta.Name, pos, dep, depPos)
			}
		}
	}
}

func TestRegisterPanicsOnDuplicate(t *testing.T) {
	saved := steps.TakeSnapshot()
	defer steps.RestoreSnapshot(saved)

	steps.RestoreSnapshot(nil)

	steps.Register(pipeline.StepMeta{Name: "dupe"}, nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Register did not panic on duplicate name")
		}
	}()

	steps.Register(pipeline.StepMeta{Name: "dupe"}, nil)
}
