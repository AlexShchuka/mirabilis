package pipeline_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func TestContractAutoStep(t *testing.T) {
	var done atomic.Bool
	step := &fakeStep{
		meta:    pipeline.Meta{Name: "auto"},
		checkFn: func(context.Context) (bool, error) { return done.Load(), nil },
		runFn: func(_ context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvLine, Step: "auto", Line: "working"}
			done.Store(true)
			return nil
		},
	}
	pipeline.Contract(t, step, nil)
}

func TestContractAlreadySatisfiedStep(t *testing.T) {
	step := &fakeStep{
		meta:    pipeline.Meta{Name: "noop"},
		checkFn: func(context.Context) (bool, error) { return true, nil },
	}
	pipeline.Contract(t, step, nil)
}

func TestContractInteractiveStep(t *testing.T) {
	var stored atomic.Value
	step := &fakeStep{
		meta:    pipeline.Meta{Name: "pick", Kind: pipeline.Interactive},
		checkFn: func(context.Context) (bool, error) { return stored.Load() != nil, nil },
		runFn: func(_ context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: "pick", Payload: []string{"x", "y"}}
			r := <-in
			if r.Cancelled {
				return pipeline.ErrCancelled
			}
			stored.Store(r.Value)
			return nil
		},
	}
	pipeline.Contract(t, step, func(payload any) pipeline.Result {
		opts, ok := payload.([]string)
		if !ok || len(opts) == 0 {
			t.Errorf("payload = %v, want options", payload)
			return pipeline.Result{Cancelled: true}
		}
		return pipeline.Result{Value: opts[0]}
	})
}

func TestContractTerminalStepExempt(t *testing.T) {
	step := &fakeStep{
		meta: pipeline.Meta{Name: "attach", Kind: pipeline.Terminal},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			t.Error("terminal step run by contract harness")
			return nil
		},
	}
	pipeline.Contract(t, step, nil)
}
