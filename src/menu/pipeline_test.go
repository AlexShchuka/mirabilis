package main

import (
	"context"
	"errors"
	"testing"
)

var errBoom = errors.New("boom")

func trivialPipeline(steps []Step) *pipeline {
	return newPipeline(context.Background(), fakeRunner{}, steps)
}

func TestPipelineOnChecked(t *testing.T) {
	tests := []struct {
		name       string
		optional   bool
		give       checkedMsg
		wantStatus stepStatus
		wantFailed bool
	}{
		{name: "satisfied -> skipped", give: checkedMsg{name: "s", satisfied: true}, wantStatus: stSkipped},
		{name: "needs run -> running", give: checkedMsg{name: "s", satisfied: false}, wantStatus: stRunning},
		{name: "error non-optional -> failed", give: checkedMsg{name: "s", err: errBoom}, wantStatus: stFailed, wantFailed: true},
		{name: "error optional -> skipped", optional: true, give: checkedMsg{name: "s", err: errBoom}, wantStatus: stSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := trivialPipeline([]Step{{Name: "s", Optional: tt.optional, Run: func(context.Context, Runner) error { return nil }}})
			p.views[0].status = stRunning
			p.onChecked(tt.give)
			if got := p.views[0].status; got != tt.wantStatus {
				t.Errorf("status = %v, want %v", got, tt.wantStatus)
			}
			if p.failed != tt.wantFailed {
				t.Errorf("failed = %v, want %v", p.failed, tt.wantFailed)
			}
		})
	}
}

func TestPipelineOnRan(t *testing.T) {
	tests := []struct {
		name       string
		optional   bool
		err        error
		wantStatus stepStatus
		wantFailed bool
	}{
		{name: "success -> done", wantStatus: stDone},
		{name: "error optional -> skipped", optional: true, err: errBoom, wantStatus: stSkipped},
		{name: "error non-optional -> failed", err: errBoom, wantStatus: stFailed, wantFailed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := trivialPipeline([]Step{{Name: "s", Optional: tt.optional}})
			p.views[0].status = stRunning
			p.onRan(ranMsg{name: "s", err: tt.err})
			if got := p.views[0].status; got != tt.wantStatus {
				t.Errorf("status = %v, want %v", got, tt.wantStatus)
			}
			if p.failed != tt.wantFailed {
				t.Errorf("failed = %v, want %v", p.failed, tt.wantFailed)
			}
		})
	}
}

func TestPipelineDrainInteractiveEmitsNeedGH(t *testing.T) {
	p := trivialPipeline([]Step{{Name: "gh", Interactive: true, Optional: true}})
	p.queue = append(p.queue, p.views[0])

	cmd := p.drainInteractive()
	if cmd == nil {
		t.Fatal("drainInteractive returned nil with a queued step")
	}
	msg, ok := cmd().(needGHMsg)
	if !ok {
		t.Fatalf("emitted %T, want needGHMsg", cmd())
	}
	if msg.name != "gh" {
		t.Errorf("needGH name = %q, want gh", msg.name)
	}
	if !p.interacting {
		t.Error("drainInteractive should mark the pipeline interacting")
	}
}

func TestPipelineInteractiveStepGoesInteracting(t *testing.T) {
	p := trivialPipeline([]Step{{Name: "gh", Interactive: true, Optional: true}})
	p.views[0].status = stRunning
	p.onChecked(checkedMsg{name: "gh", satisfied: false})
	if !p.interacting {
		t.Error("an interactive step should drive the pipeline interacting after its check")
	}
}

func TestPipelineDone(t *testing.T) {
	p := trivialPipeline([]Step{{Name: "a"}, {Name: "b"}})
	if p.done() {
		t.Error("pipeline with pending steps is not done")
	}
	p.views[0].status = stDone
	p.views[1].status = stSkipped
	if !p.done() {
		t.Error("all-resolved pipeline should be done")
	}
	p.interacting = true
	if p.done() {
		t.Error("interacting pipeline is not done")
	}
}

func TestPipelineResolved(t *testing.T) {
	p := trivialPipeline([]Step{{Name: "a"}, {Name: "b"}, {Name: "c"}})
	p.views[0].status = stDone
	p.views[1].status = stFailed
	p.views[2].status = stRunning
	if got := p.resolved(); got != 2 {
		t.Errorf("resolved = %d, want 2", got)
	}
}

func TestPipelineDepsReady(t *testing.T) {
	p := trivialPipeline([]Step{{Name: "a"}, {Name: "b", Deps: []string{"a"}}})
	if p.depsReady(p.views[1].step) {
		t.Error("b should not be ready while a is pending")
	}
	p.views[0].status = stDone
	if !p.depsReady(p.views[1].step) {
		t.Error("b should be ready after a is done")
	}
}

func TestProgressWidth(t *testing.T) {
	tests := []struct {
		give int
		want int
	}{
		{give: 10, want: 10},
		{give: 38, want: 10},
		{give: 50, want: 22},
		{give: 200, want: 50},
	}
	for _, tt := range tests {
		if got := progressWidth(tt.give); got != tt.want {
			t.Errorf("progressWidth(%d) = %d, want %d", tt.give, got, tt.want)
		}
	}
}
