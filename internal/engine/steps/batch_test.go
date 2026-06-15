package steps

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type recordedStep struct {
	mu      sync.Mutex
	name    string
	deps    []string
	startAt time.Time
	doneAt  time.Time
}

func (r *recordedStep) Meta() pipeline.Meta {
	return pipeline.Meta{Name: r.name, Deps: r.deps, Kind: pipeline.Auto}
}

func (r *recordedStep) Check(_ context.Context) (bool, error) { return false, nil }

func (r *recordedStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	r.mu.Lock()
	r.startAt = time.Now()
	r.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	r.mu.Lock()
	r.doneAt = time.Now()
	r.mu.Unlock()
	return nil
}

func TestBatchDepOrderEnforced(t *testing.T) {
	a := &recordedStep{name: "a"}
	b := &recordedStep{name: "b", deps: []string{"a"}}

	batch := newBatch("test-batch", "Test", nil, []pipeline.Command{a, b})

	out := make(chan pipeline.Event, 32)
	ctx := t.Context()
	if err := batch.Run(ctx, out, nil); err != nil {
		t.Fatalf("batch.Run: %v", err)
	}
	close(out)

	a.mu.Lock()
	aDone := a.doneAt
	a.mu.Unlock()
	b.mu.Lock()
	bStart := b.startAt
	b.mu.Unlock()

	if bStart.Before(aDone) {
		t.Errorf("step b started (%v) before step a finished (%v) — dep ordering violated", bStart, aDone)
	}
}

func TestBatchParallelStepsRunConcurrently(t *testing.T) {
	const delay = 30 * time.Millisecond
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	makeStep := func(name string) pipeline.Command {
		return &funcStep{
			name: name,
			run: func(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
				n := concurrent.Add(1)
				defer concurrent.Add(-1)
				for {
					cur := maxConcurrent.Load()
					if n <= cur || maxConcurrent.CompareAndSwap(cur, n) {
						break
					}
				}
				time.Sleep(delay)
				return nil
			},
		}
	}

	steps := []pipeline.Command{
		makeStep("p"),
		makeStep("q"),
		makeStep("r"),
	}
	batch := newBatch("test-parallel", "Parallel", nil, steps)
	out := make(chan pipeline.Event, 32)
	start := time.Now()
	if err := batch.Run(t.Context(), out, nil); err != nil {
		t.Fatalf("batch.Run: %v", err)
	}
	close(out)
	elapsed := time.Since(start)

	if maxConcurrent.Load() < 2 {
		t.Error("parallel steps did not run concurrently (max concurrent < 2)")
	}
	if elapsed > 2*delay {
		t.Errorf("parallel steps took %v, want < %v (should have run in one wave)", elapsed, 2*delay)
	}
}

type funcStep struct {
	name string
	deps []string
	run  func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error
}

func (f *funcStep) Meta() pipeline.Meta {
	return pipeline.Meta{Name: f.name, Deps: f.deps, Kind: pipeline.Auto}
}

func (f *funcStep) Check(_ context.Context) (bool, error) { return false, nil }

func (f *funcStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	return f.run(ctx, out, in)
}
