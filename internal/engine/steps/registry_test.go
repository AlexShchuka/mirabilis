package steps

import (
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestLaunchRegistry(t *testing.T) {
	t.Parallel()
	wantOuter := []struct {
		name string
		deps []string
		kind pipeline.Kind
	}{
		{name: "preflight", kind: pipeline.Auto},
		{name: "config", kind: pipeline.Interactive},
		{name: "telegram", deps: []string{"config"}, kind: pipeline.Interactive},
		{name: "claude-auth", deps: []string{"preflight"}, kind: pipeline.Terminal},
		{name: autoBatchName, deps: []string{"claude-auth", configStepName}, kind: pipeline.Auto},
		{name: "gh-auth", deps: []string{autoBatchName}, kind: pipeline.Interactive},
		{name: "attach", deps: []string{"claude-auth", autoBatchName}, kind: pipeline.Terminal},
	}
	wantInner := []struct {
		name string
		deps []string
		kind pipeline.Kind
	}{
		{name: "image", deps: []string{"preflight", configStepName}, kind: pipeline.Auto},
		{name: "container", deps: []string{"image"}, kind: pipeline.Auto},
		{name: "provision-create", deps: []string{"container"}, kind: pipeline.Auto},
		{name: "provision-start", deps: []string{"provision-create"}, kind: pipeline.Auto},
		{name: "plugins", deps: []string{"provision-start", configStepName}, kind: pipeline.Auto},
		{name: "skills", deps: []string{"provision-start", configStepName}, kind: pipeline.Auto},
		{name: "harness", deps: []string{"provision-start"}, kind: pipeline.Auto},
	}

	steps := Launch(newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore()))
	if len(steps) != len(wantOuter) {
		t.Fatalf("got %d outer steps, want %d", len(steps), len(wantOuter))
	}
	for i, w := range wantOuter {
		m := steps[i].Meta()
		if m.Name != w.name {
			t.Fatalf("outer[%d] = %q, want %q", i, m.Name, w.name)
		}
		if !slices.Equal(m.Deps, w.deps) {
			t.Errorf("outer %s: deps = %v, want %v", w.name, m.Deps, w.deps)
		}
		if m.Kind != w.kind {
			t.Errorf("outer %s: kind = %v, want %v", w.name, m.Kind, w.kind)
		}
		if m.Title == "" {
			t.Errorf("outer %s: empty title", w.name)
		}
	}

	batch := steps[4].(*batchStep)
	if len(batch.cmds) != len(wantInner) {
		t.Fatalf("batch has %d inner steps, want %d", len(batch.cmds), len(wantInner))
	}
	for i, w := range wantInner {
		m := batch.cmds[i].Meta()
		if m.Name != w.name {
			t.Fatalf("inner[%d] = %q, want %q", i, m.Name, w.name)
		}
		if !slices.Equal(m.Deps, w.deps) {
			t.Errorf("inner %s: deps = %v, want %v", w.name, m.Deps, w.deps)
		}
		if m.Kind != w.kind {
			t.Errorf("inner %s: kind = %v, want %v", w.name, m.Kind, w.kind)
		}
		if m.Title == "" {
			t.Errorf("inner %s: empty title", w.name)
		}
	}
}

func TestLaunchRegistersWithPipeline(t *testing.T) {
	t.Parallel()
	steps := Launch(newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore()))
	if _, err := pipeline.New(nil, steps...); err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}
}
