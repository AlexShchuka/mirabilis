package plugins

import (
	"context"
	"reflect"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func bashScript(s string) []string { return []string{"bash", "-lc", s} }

func fakeInstaller(f *exec.Fake, settings *map[string]any, exists bool) Installer {
	return Installer{
		ScriptOK: func(ctx context.Context, s string) bool {
			_, err := exec.Run(ctx, f, exec.Spec{Argv: bashScript(s)})
			return err == nil
		},
		Output: func(ctx context.Context, argv ...string) (string, error) {
			return exec.Run(ctx, f, exec.Spec{Argv: argv})
		},
		Stream: func(ctx context.Context, _ string, _ chan<- pipeline.Event, argv ...string) error {
			for ev := range f.Stream(ctx, exec.Spec{Argv: argv}) {
				if ev.Kind == exec.KindExited {
					return ev.Err
				}
			}
			return nil
		},
		Script: func(ctx context.Context, _ string, _ chan<- pipeline.Event, s string) error {
			for ev := range f.Stream(ctx, exec.Spec{Argv: bashScript(s)}) {
				if ev.Kind == exec.KindExited {
					return ev.Err
				}
			}
			return nil
		},
		Settings: SettingsIO{
			Read:   func() (map[string]any, error) { return *settings, nil },
			Write:  func(m map[string]any) error { *settings = m; return nil },
			Exists: func() bool { return exists },
		},
	}
}

func apply(t *testing.T, i Installer, plan Plan) error {
	t.Helper()
	out := make(chan pipeline.Event, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range out {
		}
	}()
	err := i.Apply(t.Context(), out, plan)
	close(out)
	<-done
	return err
}

func samplePlan() Plan {
	return Plan{
		Marketplaces: []string{"m1"},
		Units:        []string{"alpha@1.0"},
		Enabled:      map[string]any{NeuroMatrix: true, "alpha@1.0": true},
		Configured:   true,
	}
}

func TestBuildPlanExcludesDisabledEnablesNeuroMatrix(t *testing.T) {
	t.Parallel()
	plan := BuildPlan([]string{"alpha@1.0", "beta"}, map[string]bool{"beta": true}, false, []string{"m1"})
	if !plan.Configured {
		t.Fatal("Configured should be true")
	}
	if !reflect.DeepEqual(plan.Units, []string{"alpha@1.0"}) {
		t.Errorf("Units = %#v, want [alpha@1.0]", plan.Units)
	}
	want := map[string]any{NeuroMatrix: true, "alpha@1.0": true}
	if !reflect.DeepEqual(plan.Enabled, want) {
		t.Errorf("Enabled = %#v, want %#v", plan.Enabled, want)
	}
}

func TestBuildPlanHarnessSkipExcludesNeuroMatrix(t *testing.T) {
	t.Parallel()
	plan := BuildPlan([]string{"alpha@1.0"}, nil, true, nil)
	if _, ok := plan.Enabled[NeuroMatrix]; ok {
		t.Fatalf("neuro-matrix must be excluded when harness skipped: %#v", plan.Enabled)
	}
}

func TestBuildPlanEmptyCatalogNotConfigured(t *testing.T) {
	t.Parallel()
	plan := BuildPlan(nil, nil, false, nil)
	if plan.Configured {
		t.Fatal("Configured should be false for empty catalog")
	}
	if plan.Units != nil {
		t.Errorf("Units = %#v, want nil", plan.Units)
	}
}

func TestSatisfiedWhenListedAndEnabledMatch(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect(bashScript("command -v claude"), "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "alpha 1.0 enabled", nil)
	settings := map[string]any{"enabledPlugins": map[string]any{NeuroMatrix: true, "alpha@1.0": true}}
	ok, err := fakeInstaller(f, &settings, true).Satisfied(t.Context(), samplePlan())
	if err != nil || !ok {
		t.Fatalf("Satisfied = %v, %v; want true, nil", ok, err)
	}
}

func TestSatisfiedFalseWhenNotListed(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect(bashScript("command -v claude"), "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "", nil)
	settings := map[string]any{}
	ok, _ := fakeInstaller(f, &settings, true).Satisfied(t.Context(), samplePlan())
	if ok {
		t.Fatal("Satisfied true but alpha not listed")
	}
}

func TestApplyInstallsMissingAddsMarketplacesWritesEnabled(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect(bashScript("command -v claude"), "", nil)
	f.Expect(bashScript(`mkdir -p "$HOME/.cache/tmp"`), "", nil)
	f.Expect([]string{"claude", "plugin", "marketplace", "add", "m1"}, "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "", nil)
	f.Expect(bashScript(`TMPDIR="$HOME/.cache/tmp" claude plugin install "alpha@1.0" --scope user`), "", nil)
	settings := map[string]any{}
	if err := apply(t, fakeInstaller(f, &settings, true), samplePlan()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("unused stubs: %d", n)
	}
	want := map[string]any{NeuroMatrix: true, "alpha@1.0": true}
	if got, _ := settings["enabledPlugins"].(map[string]any); !reflect.DeepEqual(got, want) {
		t.Errorf("enabledPlugins = %#v, want %#v", got, want)
	}
}

func TestApplySkipsListed(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect(bashScript("command -v claude"), "", nil)
	f.Expect(bashScript(`mkdir -p "$HOME/.cache/tmp"`), "", nil)
	f.Expect([]string{"claude", "plugin", "marketplace", "add", "m1"}, "", nil)
	f.Expect([]string{"claude", "plugin", "list"}, "alpha 1.0 enabled", nil)
	settings := map[string]any{}
	if err := apply(t, fakeInstaller(f, &settings, true), samplePlan()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("install ran for already-listed plugin; unused stubs: %d", n)
	}
}

func TestClaudeAbsentGraceful(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	settings := map[string]any{}
	ok, err := fakeInstaller(f, &settings, true).Satisfied(t.Context(), samplePlan())
	if err != nil || !ok {
		t.Fatalf("Satisfied with claude absent = %v, %v; want true, nil", ok, err)
	}
	if err := apply(t, fakeInstaller(f, &settings, true), samplePlan()); err != nil {
		t.Fatalf("Apply with claude absent should noop: %v", err)
	}
}

func TestNotConfiguredNoop(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect(bashScript("command -v claude"), "", nil)
	f.Expect(bashScript("command -v claude"), "", nil)
	settings := map[string]any{}
	plan := Plan{Configured: false}
	ok, _ := fakeInstaller(f, &settings, true).Satisfied(t.Context(), plan)
	if !ok {
		t.Fatal("Satisfied should be true when not configured")
	}
	if err := apply(t, fakeInstaller(f, &settings, true), plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("plugin list/install ran when not configured; unused stubs: %d", n)
	}
}
