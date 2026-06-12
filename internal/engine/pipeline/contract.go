package pipeline

import (
	"context"
	"testing"
	"time"
)

func Contract(t *testing.T, step Command, resolve func(payload any) Result) {
	t.Helper()
	m := step.Meta()
	if m.Kind == Terminal {
		t.Skipf("pipeline: step %q is terminal: exempt from the idempotency contract", m.Name)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	satisfied, err := step.Check(ctx)
	t.Logf("pipeline: contract %q: initial check satisfied=%v err=%v", m.Name, satisfied, err)

	out := make(chan Event, 16)
	in := make(chan Result)
	resolved := make(chan struct{})
	go func() {
		defer close(resolved)
		for ev := range out {
			if ev.Kind != EvWaiting {
				continue
			}
			r := Result{}
			if resolve != nil {
				r = resolve(ev.Payload)
			}
			select {
			case in <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	runErr := step.Run(ctx, out, in)
	close(out)
	<-resolved
	if runErr != nil {
		t.Fatalf("pipeline: contract %q: run: %v", m.Name, runErr)
	}
	satisfied, err = step.Check(ctx)
	if err != nil {
		t.Fatalf("pipeline: contract %q: check after run: %v", m.Name, err)
	}
	if !satisfied {
		t.Fatalf("pipeline: contract %q: check not satisfied after run (invariant I3)", m.Name)
	}
}
