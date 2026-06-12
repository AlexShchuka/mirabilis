package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func TestHandleKeyTab(t *testing.T) {
	gate := make(chan struct{})
	step := &syncCommand{
		meta: pipeline.Meta{Name: "tab-step", Title: "Tab Step", Kind: pipeline.Auto},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			select {
			case <-gate:
			case <-ctx.Done():
			}
			return nil
		},
	}
	f := newFakeFacade([]pipeline.Command{step})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Tab Step"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("commands"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	close(gate)
}

func TestHandleKeyQAtDepthOne(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("mirabilis"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})

	if err := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)); err != nil {
		t.Logf("FinalModel: %v", err)
	}
}

func TestHandleKeyEscAtDepthOneRoutesThroughMenu(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("mirabilis"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	if err := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)); err != nil {
		t.Logf("FinalModel: %v", err)
	}
}

func TestHandlePipelineEventEvFailed(t *testing.T) {
	gate := make(chan struct{})
	failStep := &syncCommand{
		meta:    pipeline.Meta{Name: "fail-step", Title: "Fail Step", Kind: pipeline.Auto},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			<-gate
			return errors.New("step error")
		},
	}

	f := newFakeFacade([]pipeline.Command{failStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Fail Step"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	close(gate)

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.WelcomeHint))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestHandlePipelineEventEvSkipped(t *testing.T) {
	gate := make(chan struct{})
	depStep := &syncCommand{
		meta:    pipeline.Meta{Name: "dep-step", Title: "Dep Step", Kind: pipeline.Auto},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			<-gate
			return errors.New("dep failed")
		},
	}
	skipStep := &syncCommand{
		meta: pipeline.Meta{
			Name:  "skip-step",
			Title: "Skip Step",
			Kind:  pipeline.Auto,
			Deps:  []string{"dep-step"},
		},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn:   func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error { return nil },
	}

	f := newFakeFacade([]pipeline.Command{depStep, skipStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Dep Step")) &&
			bytes.Contains(bts, []byte("Skip Step"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	close(gate)

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.WelcomeHint))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestHandlePipelineEventEvDone(t *testing.T) {
	satisfiedStep := &syncCommand{
		meta:    pipeline.Meta{Name: "done-step", Title: "Done Step", Kind: pipeline.Auto},
		checkFn: func(_ context.Context) (bool, error) { return true, nil },
		runFn:   func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error { return nil },
	}

	f := newFakeFacade([]pipeline.Command{satisfiedStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("mirabilis"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestHandlePipelineDoneNotFailed(t *testing.T) {
	quickStep := &syncCommand{
		meta:    pipeline.Meta{Name: "quick", Title: "Quick", Kind: pipeline.Auto},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn:   func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error { return nil },
	}

	f := newFakeFacade([]pipeline.Command{quickStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.WelcomeHint))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	time.Sleep(50 * time.Millisecond)
	out := tm.Output()
	var acc bytes.Buffer
	_, _ = io.Copy(&acc, out)
	if bytes.Contains(acc.Bytes(), []byte(uistr.NoticeLaunchFailed)) {
		t.Error("notice shows launch failed, but pipeline succeeded")
	}
}

func TestHandleHarnessDoneError(t *testing.T) {
	f := newFakeFacade(nil)
	f.harnessCurrent = "on"
	f.harnessApplyErr = errors.New("apply harness boom")
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "harness"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.FormTitleHarness))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenResult{Value: "on"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("harness boom"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestHandleWaitingDefaultPayload(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)
	unknownPayloadStep := &syncCommand{
		meta:    pipeline.Meta{Name: "unknown-payload", Title: "Unknown Payload", Kind: pipeline.Interactive},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind:    pipeline.EvWaiting,
				Step:    "unknown-payload",
				Payload: struct{ X int }{X: 42},
			}
			r := <-in
			resumedWith <- r
			return nil
		},
	}

	f := newFakeFacade([]pipeline.Command{unknownPayloadStep})
	tm := newApp(t, f)
	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Unknown Payload"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenPop{})

	select {
	case r := <-resumedWith:
		if !r.Cancelled {
			t.Error("Resume was not Cancelled after ScreenPop with default payload, want Cancelled=true")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline not resumed after ScreenPop")
	}
}

func TestStartLaunchAlreadyRunning(t *testing.T) {
	gate := make(chan struct{})
	runningStep := &syncCommand{
		meta: pipeline.Meta{Name: "running-step", Title: "Running Step", Kind: pipeline.Auto},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			select {
			case <-gate:
			case <-ctx.Done():
			}
			return nil
		},
	}

	f := newFakeFacade([]pipeline.Command{runningStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Running Step"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.MenuChosen{Action: "launch"})

	time.Sleep(200 * time.Millisecond)

	close(gate)
}

func TestStartLaunchPipelineNewError(t *testing.T) {
	dupStep1 := &syncCommand{
		meta: pipeline.Meta{Name: "dup", Title: "Dup A", Kind: pipeline.Auto},
	}
	dupStep2 := &syncCommand{
		meta: pipeline.Meta{Name: "dup", Title: "Dup B", Kind: pipeline.Auto},
	}

	f := newFakeFacade([]pipeline.Command{dupStep1, dupStep2})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.NoticeLaunchErrPrefix))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestHandleMenuChosenDefault(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "unknown-action-xyz"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("mirabilis"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestHandleScreenPopWithMenuAction(t *testing.T) {
	f := newFakeFacade(nil)
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "reset"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Delete everything"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.WelcomeHint))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestHandleScreenPopWithPipelineWaiting(t *testing.T) {
	resumedWith := make(chan pipeline.Result, 1)
	waitStep := &syncCommand{
		meta:    pipeline.Meta{Name: "wait-step-pop", Title: "Wait Pop", Kind: pipeline.Interactive},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{
				Kind:    pipeline.EvWaiting,
				Step:    "wait-step-pop",
				Payload: struct{ X int }{X: 1},
			}
			r := <-in
			resumedWith <- r
			return nil
		},
	}

	f := newFakeFacade([]pipeline.Command{waitStep})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Wait Pop"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(bus.ScreenPop{})

	select {
	case r := <-resumedWith:
		if !r.Cancelled {
			t.Error("Resume was not Cancelled, want Cancelled=true")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline not resumed via ScreenPop")
	}
}

func TestWatchStatusClosedChannel(t *testing.T) {
	f := newFakeFacade(nil)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	a := newApp(t, f)

	close(f.statusCh)

	teatest.WaitFor(t, a.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("mirabilis"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}
