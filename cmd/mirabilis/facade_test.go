package main

import (
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/steps"
)

func stepNames(cmds []pipeline.Command) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.Meta().Name
	}
	return out
}

func TestFacadeLaunchStepsReviewSelection(t *testing.T) {
	f := &facade{repo: t.TempDir()}

	def := stepNames(f.LaunchSteps())
	wantLaunch := stepNames(steps.Launch(f.deps))
	if len(def) != len(wantLaunch) || def[0] != wantLaunch[0] {
		t.Fatalf("default LaunchSteps = %v, want Launch set %v", def, wantLaunch)
	}
	for _, n := range def {
		if n == "review-harvest" || n == "review-present" {
			t.Fatalf("default launch selected a review step: %v", def)
		}
	}

	f.SetReviewMode(true)
	got := stepNames(f.LaunchSteps())
	want := stepNames(steps.Review(f.deps))
	if len(got) != len(want) {
		t.Fatalf("review LaunchSteps = %v, want Review set %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("review LaunchSteps[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}

	f.SetReviewMode(false)
	if back := stepNames(f.LaunchSteps()); len(back) != len(wantLaunch) {
		t.Errorf("after clearing review mode, LaunchSteps = %v, want Launch set", back)
	}
}
