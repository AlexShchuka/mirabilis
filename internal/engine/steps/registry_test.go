package steps

import (
	"slices"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestLaunchRegistry(t *testing.T) {
	t.Parallel()
	want := []struct {
		name     string
		deps     []string
		kind     pipeline.Kind
		timeout  time.Duration
		retry    pipeline.RetryPolicy
		optional bool
	}{
		{name: "preflight", kind: pipeline.Auto, timeout: 90 * time.Second},
		{name: "claude-auth", deps: []string{"preflight"}, kind: pipeline.Terminal},
		{name: "config", kind: pipeline.Interactive},
		{name: "telegram", deps: []string{"config"}, kind: pipeline.Interactive, optional: true},
		{name: "image", deps: []string{"preflight", "config"}, kind: pipeline.Auto, timeout: 15 * time.Minute},
		{
			name: "container", deps: []string{"image"}, kind: pipeline.Auto, timeout: 3 * time.Minute,
			retry: pipeline.RetryPolicy{Attempts: 2, Delay: 2 * time.Second},
		},
		{
			name: "provision-create", deps: []string{"container"}, kind: pipeline.Auto, timeout: 5 * time.Minute,
			retry: pipeline.RetryPolicy{Attempts: 2, Delay: 2 * time.Second},
		},
		{
			name: "provision-start", deps: []string{"provision-create"}, kind: pipeline.Auto, timeout: 5 * time.Minute,
			retry: pipeline.RetryPolicy{Attempts: 2, Delay: 2 * time.Second},
		},
		{name: "gh-auth", deps: []string{"container"}, kind: pipeline.Interactive},
		{
			name: "plugins", deps: []string{"provision-start", "config"}, kind: pipeline.Auto,
			timeout: 5 * time.Minute, optional: true,
		},
		{
			name: "skills", deps: []string{"provision-start", "config"}, kind: pipeline.Auto,
			timeout: 5 * time.Minute, optional: true,
		},
		{
			name: "harness", deps: []string{"provision-start"}, kind: pipeline.Auto,
			timeout: 5 * time.Minute, optional: true,
		},
		{name: "attach", deps: []string{"claude-auth", "provision-start"}, kind: pipeline.Terminal},
	}
	steps := Launch(newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore()))
	if len(steps) != len(want) {
		t.Fatalf("got %d steps, want %d", len(steps), len(want))
	}
	for i, w := range want {
		m := steps[i].Meta()
		if m.Name != w.name {
			t.Fatalf("step[%d] = %q, want %q", i, m.Name, w.name)
		}
		if !slices.Equal(m.Deps, w.deps) {
			t.Errorf("%s: deps = %v, want %v", w.name, m.Deps, w.deps)
		}
		if m.Kind != w.kind {
			t.Errorf("%s: kind = %v, want %v", w.name, m.Kind, w.kind)
		}
		if m.Timeout != w.timeout {
			t.Errorf("%s: timeout = %v, want %v", w.name, m.Timeout, w.timeout)
		}
		if m.Retry != w.retry {
			t.Errorf("%s: retry = %+v, want %+v", w.name, m.Retry, w.retry)
		}
		if m.Optional != w.optional {
			t.Errorf("%s: optional = %v, want %v", w.name, m.Optional, w.optional)
		}
		if m.Title == "" {
			t.Errorf("%s: empty title", w.name)
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
