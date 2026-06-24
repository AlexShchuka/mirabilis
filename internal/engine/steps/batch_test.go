package steps

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type fakeCmd struct {
	name      string
	deps      []string
	optional  bool
	satisfied bool
	runErr    error

	started  func()
	finished func()
}

func (c fakeCmd) cmd() pipeline.Command {
	cp := c
	return &cp
}

func (c *fakeCmd) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     c.name,
		Title:    c.name,
		Deps:     c.deps,
		Kind:     pipeline.Auto,
		Optional: c.optional,
	}
}

func (c *fakeCmd) Check(context.Context) (bool, error) { return c.satisfied, nil }

func (c *fakeCmd) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if c.started != nil {
		c.started()
	}
	if c.finished != nil {
		defer c.finished()
	}
	out <- pipeline.Event{Kind: pipeline.EvLine, Step: c.name, Line: "working"}
	if c.runErr != nil {
		return c.runErr
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func drain(t *testing.T, b *batchStep) ([]pipeline.Event, error) {
	t.Helper()
	out := make(chan pipeline.Event, 256)
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Run(t.Context(), out, nil)
		close(out)
	}()
	var got []pipeline.Event
	for ev := range out {
		got = append(got, ev)
	}
	return got, <-errCh
}

func TestBatchOuterDeps(t *testing.T) {
	t.Parallel()
	b := newBatch("apply-batch", "Apply", []pipeline.Command{
		&fakeCmd{name: "a", deps: []string{"provision-start", "config"}},
		&fakeCmd{name: "b", deps: []string{"provision-start", "a"}},
	})
	want := []string{"config", "provision-start"}
	if got := b.Meta().Deps; !slices.Equal(got, want) {
		t.Fatalf("outer deps = %v, want %v (inner dep 'a' must be excluded)", got, want)
	}
}

func TestBatchRespectsInnerDeps(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var order []string
	mark := func(name string) func() {
		return func() {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}
	b := newBatch("apply-batch", "Apply", []pipeline.Command{
		fakeCmd{name: "first", finished: mark("first")}.cmd(),
		fakeCmd{name: "second", deps: []string{"first"}, started: mark("second")}.cmd(),
	})
	if _, err := drain(t, b); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !slices.Equal(order, []string{"first", "second"}) {
		t.Fatalf("order = %v, want first finishing before second starts", order)
	}
}

func TestBatchParallelGrouping(t *testing.T) {
	t.Parallel()
	var inFlight, peak int32
	bump := func() {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	drop := func() { atomic.AddInt32(&inFlight, -1) }
	b := newBatch("apply-batch", "Apply", []pipeline.Command{
		fakeCmd{name: "x", started: bump, finished: drop}.cmd(),
		fakeCmd{name: "y", started: bump, finished: drop}.cmd(),
		fakeCmd{name: "z", started: bump, finished: drop}.cmd(),
	})
	if _, err := drain(t, b); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if peak < 2 {
		t.Fatalf("peak concurrency = %d, want >= 2 (steps should run in parallel)", peak)
	}
}

func TestBatchErrorPropagation(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	b := newBatch("apply-batch", "Apply", []pipeline.Command{
		fakeCmd{name: "ok"}.cmd(),
		fakeCmd{name: "bad", runErr: boom}.cmd(),
	})
	got, err := drain(t, b)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
	if !hasEvent(got, pipeline.EvFailed, "bad") {
		t.Fatalf("missing EvFailed for 'bad'; events = %+v", got)
	}
}

func TestBatchOptionalErrorSwallowed(t *testing.T) {
	t.Parallel()
	b := newBatch("apply-batch", "Apply", []pipeline.Command{
		fakeCmd{name: "opt", optional: true, runErr: errors.New("ignored")}.cmd(),
		fakeCmd{name: "ok"}.cmd(),
	})
	got, err := drain(t, b)
	if err != nil {
		t.Fatalf("optional failure must not abort batch: %v", err)
	}
	if !hasEvent(got, pipeline.EvSkipped, "opt") {
		t.Fatalf("optional failure should emit EvSkipped; events = %+v", got)
	}
}

func TestBatchSatisfiedShortCircuits(t *testing.T) {
	t.Parallel()
	var ran atomic.Bool
	b := newBatch("apply-batch", "Apply", []pipeline.Command{
		fakeCmd{name: "done", satisfied: true, started: func() { ran.Store(true) }}.cmd(),
	})
	got, err := drain(t, b)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ran.Load() {
		t.Fatal("satisfied step must not Run")
	}
	if !hasEvent(got, pipeline.EvDone, "done") {
		t.Fatalf("satisfied step should emit EvDone; events = %+v", got)
	}
}

func TestBatchCheckGatesWholeBatch(t *testing.T) {
	t.Parallel()
	all := newBatch("b", "B", []pipeline.Command{
		fakeCmd{name: "p", satisfied: true}.cmd(),
		fakeCmd{name: "q", satisfied: true}.cmd(),
	})
	ok, err := all.Check(t.Context())
	if err != nil || !ok {
		t.Fatalf("all-satisfied batch Check = (%v, %v), want (true, nil)", ok, err)
	}
	partial := newBatch("b", "B", []pipeline.Command{
		fakeCmd{name: "p", satisfied: true}.cmd(),
		fakeCmd{name: "q", satisfied: false}.cmd(),
	})
	ok, err = partial.Check(t.Context())
	if err != nil || ok {
		t.Fatalf("partially-satisfied batch Check = (%v, %v), want (false, nil)", ok, err)
	}
}

func hasEvent(evs []pipeline.Event, kind pipeline.EventKind, step string) bool {
	for _, ev := range evs {
		if ev.Kind == kind && ev.Step == step {
			return true
		}
	}
	return false
}
