package pipeline_test

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type fakeStep struct {
	meta    pipeline.Meta
	checkFn func(ctx context.Context) (bool, error)
	runFn   func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error
}

func (f *fakeStep) Meta() pipeline.Meta { return f.meta }

func (f *fakeStep) Check(ctx context.Context) (bool, error) {
	if f.checkFn == nil {
		return false, nil
	}
	return f.checkFn(ctx)
}

func (f *fakeStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	if f.runFn == nil {
		return nil
	}
	return f.runFn(ctx, out, in)
}

type recorder struct {
	mu    sync.Mutex
	order []string
}

func (r *recorder) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append(r.order, name)
}

func (r *recorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

func autoStep(name string, rec *recorder, deps ...string) *fakeStep {
	return &fakeStep{
		meta: pipeline.Meta{Name: name, Deps: deps},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			rec.add(name)
			return nil
		},
	}
}

func waitingStep(name string, meta pipeline.Meta, got *atomic.Value) *fakeStep {
	meta.Name = name
	return &fakeStep{
		meta: meta,
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvWaiting, Payload: "prompt"}
			r := <-in
			if r.Cancelled {
				return pipeline.ErrCancelled
			}
			if got != nil {
				got.Store(r.Value)
			}
			return nil
		},
	}
}

func runPipeline(t *testing.T, ctx context.Context, p *pipeline.Pipeline, react func(ev pipeline.Event)) ([]pipeline.Event, error) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx) }()
	var evs []pipeline.Event
	for ev := range p.Events() {
		evs = append(evs, ev)
		if react != nil {
			react(ev)
		}
	}
	select {
	case err := <-errCh:
		return evs, err
	case <-time.After(10 * time.Second):
		t.Fatal("pipeline run did not return")
		return nil, nil
	}
}

func stepKinds(evs []pipeline.Event, step string) []pipeline.EventKind {
	var out []pipeline.EventKind
	for _, ev := range evs {
		if ev.Step == step {
			out = append(out, ev.Kind)
		}
	}
	return out
}

func findEvent(evs []pipeline.Event, step string, kind pipeline.EventKind) (pipeline.Event, bool) {
	for _, ev := range evs {
		if ev.Step == step && ev.Kind == kind {
			return ev, true
		}
	}
	return pipeline.Event{}, false
}

func lastEvent(t *testing.T, evs []pipeline.Event) pipeline.Event {
	t.Helper()
	if len(evs) == 0 {
		t.Fatal("no events")
	}
	return evs[len(evs)-1]
}

func TestNewValidation(t *testing.T) {
	mk := func(name string, deps ...string) pipeline.Command {
		return &fakeStep{meta: pipeline.Meta{Name: name, Deps: deps}}
	}
	tests := []struct {
		name    string
		steps   []pipeline.Command
		wantErr string
	}{
		{name: "valid", steps: []pipeline.Command{mk("a"), mk("b", "a")}},
		{name: "empty name", steps: []pipeline.Command{mk("")}, wantErr: "empty name"},
		{name: "duplicate name", steps: []pipeline.Command{mk("a"), mk("a")}, wantErr: "duplicate"},
		{name: "unknown dep", steps: []pipeline.Command{mk("a", "ghost")}, wantErr: "unknown dependency"},
		{name: "cycle", steps: []pipeline.Command{mk("a", "b"), mk("b", "a")}, wantErr: "cycle"},
		{name: "self dep", steps: []pipeline.Command{mk("a", "a")}, wantErr: "cycle"},
		{name: "forward dep", steps: []pipeline.Command{mk("a", "b"), mk("b")}, wantErr: "cycle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pipeline.New(nil, tt.steps...)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("New() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("New() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSequentialOrder(t *testing.T) {
	rec := &recorder{}
	p, err := pipeline.New(nil, autoStep("a", rec), autoStep("b", rec, "a"), autoStep("c", rec))
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"a", "b", "c"}
	got := rec.names()
	if len(got) != len(want) {
		t.Fatalf("ran %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ran %v, want %v", got, want)
		}
	}
	var started []string
	for _, ev := range evs {
		if ev.Kind == pipeline.EvStepStarted {
			started = append(started, ev.Step)
		}
	}
	for i := range want {
		if started[i] != want[i] {
			t.Fatalf("started %v, want %v", started, want)
		}
	}
	if last := lastEvent(t, evs); last.Kind != pipeline.EvPipelineDone || last.Failed {
		t.Fatalf("last event = %+v, want EvPipelineDone Failed=false", last)
	}
}

func TestAlreadySatisfied(t *testing.T) {
	rec := &recorder{}
	sat := &fakeStep{
		meta:    pipeline.Meta{Name: "sat"},
		checkFn: func(context.Context) (bool, error) { return true, nil },
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			t.Error("run called on satisfied step")
			return nil
		},
	}
	p, err := pipeline.New(nil, sat, autoStep("dep", rec, "sat"))
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	kinds := stepKinds(evs, "sat")
	if len(kinds) != 1 || kinds[0] != pipeline.EvDone {
		t.Fatalf("sat events = %v, want [EvDone]", kinds)
	}
	done, _ := findEvent(evs, "sat", pipeline.EvDone)
	if done.Line != pipeline.LineSatisfied {
		t.Fatalf("done line = %q, want %q", done.Line, pipeline.LineSatisfied)
	}
	if got := rec.names(); len(got) != 1 || got[0] != "dep" {
		t.Fatalf("ran %v, want [dep]", got)
	}
}

func TestOptionalFailureContinues(t *testing.T) {
	rec := &recorder{}
	errBoom := errors.New("boom")
	opt := &fakeStep{
		meta: pipeline.Meta{Name: "opt", Optional: true},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			return errBoom
		},
	}
	p, err := pipeline.New(nil, opt, autoStep("dep", rec, "opt"), autoStep("other", rec))
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	skipped, ok := findEvent(evs, "opt", pipeline.EvSkipped)
	if !ok || !errors.Is(skipped.Err, errBoom) {
		t.Fatalf("opt skipped = %+v ok=%v, want EvSkipped with boom", skipped, ok)
	}
	if _, ok := findEvent(evs, "opt", pipeline.EvFailed); ok {
		t.Fatal("optional failure must not emit EvFailed")
	}
	got := rec.names()
	if len(got) != 2 || got[0] != "dep" || got[1] != "other" {
		t.Fatalf("ran %v, want [dep other]", got)
	}
	if last := lastEvent(t, evs); last.Failed {
		t.Fatalf("pipeline done failed = true, want false")
	}
}

func TestRequiredFailureCascade(t *testing.T) {
	rec := &recorder{}
	errBoom := errors.New("boom")
	bad := &fakeStep{
		meta: pipeline.Meta{Name: "bad"},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			return errBoom
		},
	}
	p, err := pipeline.New(nil,
		bad,
		autoStep("child", rec, "bad"),
		autoStep("free", rec),
		autoStep("grandchild", rec, "child"),
	)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if !errors.Is(err, errBoom) {
		t.Fatalf("run error = %v, want boom", err)
	}
	failed, ok := findEvent(evs, "bad", pipeline.EvFailed)
	if !ok || !errors.Is(failed.Err, errBoom) {
		t.Fatalf("bad failed = %+v ok=%v, want EvFailed with boom", failed, ok)
	}
	childKinds := stepKinds(evs, "child")
	if len(childKinds) != 1 || childKinds[0] != pipeline.EvSkipped {
		t.Fatalf("child events = %v, want [EvSkipped]", childKinds)
	}
	skipped, _ := findEvent(evs, "child", pipeline.EvSkipped)
	if !strings.Contains(skipped.Line, "bad") {
		t.Fatalf("skip line = %q, want mention of failed dep", skipped.Line)
	}
	got := rec.names()
	if len(got) != 2 || got[0] != "free" || got[1] != "grandchild" {
		t.Fatalf("ran %v, want [free grandchild]", got)
	}
	if last := lastEvent(t, evs); !last.Failed {
		t.Fatal("pipeline done failed = false, want true")
	}
}

func TestCheckError(t *testing.T) {
	tests := []struct {
		name     string
		optional bool
		wantKind pipeline.EventKind
		wantErr  bool
	}{
		{name: "required", wantKind: pipeline.EvFailed, wantErr: true},
		{name: "optional", optional: true, wantKind: pipeline.EvSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errCheck := errors.New("check broke")
			step := &fakeStep{
				meta:    pipeline.Meta{Name: "s", Optional: tt.optional},
				checkFn: func(context.Context) (bool, error) { return false, errCheck },
				runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
					t.Error("run called after check error")
					return nil
				},
			}
			p, err := pipeline.New(nil, step)
			if err != nil {
				t.Fatal(err)
			}
			evs, err := runPipeline(t, context.Background(), p, nil)
			if tt.wantErr != (err != nil) {
				t.Fatalf("run error = %v, want error=%v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, errCheck) {
				t.Fatalf("run error = %v, want check error", err)
			}
			ev, ok := findEvent(evs, "s", tt.wantKind)
			if !ok || !errors.Is(ev.Err, errCheck) {
				t.Fatalf("event = %+v ok=%v, want kind %v with check error", ev, ok, tt.wantKind)
			}
		})
	}
}

func TestForwardStreaming(t *testing.T) {
	fake := exec.NewFake().Expect([]string{"echo", "hi"}, "hi\nbye", nil)
	step := &fakeStep{
		meta: pipeline.Meta{Name: "stream"},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			for ev := range fake.Stream(ctx, exec.Spec{Argv: []string{"echo", "hi"}}) {
				pipeline.Forward("stream", out, ev)
			}
			return nil
		},
	}
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	spawn, ok := findEvent(evs, "stream", pipeline.EvSpawn)
	if !ok || len(spawn.Argv) != 2 || spawn.Argv[0] != "echo" || spawn.Argv[1] != "hi" {
		t.Fatalf("spawn = %+v ok=%v, want EvSpawn argv [echo hi]", spawn, ok)
	}
	var lines []string
	for _, ev := range evs {
		if ev.Step == "stream" && ev.Kind == pipeline.EvLine {
			lines = append(lines, ev.Line)
		}
	}
	if len(lines) != 2 || lines[0] != "hi" || lines[1] != "bye" {
		t.Fatalf("lines = %v, want [hi bye]", lines)
	}
}

func TestResumeInteractive(t *testing.T) {
	var got atomic.Value
	step := waitingStep("ask", pipeline.Meta{Kind: pipeline.Interactive}, &got)
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, func(ev pipeline.Event) {
		if ev.Kind == pipeline.EvWaiting {
			if err := p.Resume("ask", pipeline.Result{Value: "v"}); err != nil {
				t.Errorf("resume: %v", err)
			}
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waiting, ok := findEvent(evs, "ask", pipeline.EvWaiting)
	if !ok || waiting.Payload != "prompt" {
		t.Fatalf("waiting = %+v ok=%v, want payload prompt", waiting, ok)
	}
	if got.Load() != "v" {
		t.Fatalf("step got %v, want v", got.Load())
	}
	if _, ok := findEvent(evs, "ask", pipeline.EvDone); !ok {
		t.Fatal("no EvDone for ask")
	}
}

func TestResumeCancelled(t *testing.T) {
	tests := []struct {
		name       string
		optional   bool
		wantKind   pipeline.EventKind
		wantFailed bool
	}{
		{name: "required", wantKind: pipeline.EvFailed, wantFailed: true},
		{name: "optional", optional: true, wantKind: pipeline.EvSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := waitingStep("ask", pipeline.Meta{Kind: pipeline.Interactive, Optional: tt.optional}, nil)
			p, err := pipeline.New(nil, step)
			if err != nil {
				t.Fatal(err)
			}
			evs, err := runPipeline(t, context.Background(), p, func(ev pipeline.Event) {
				if ev.Kind == pipeline.EvWaiting {
					if err := p.Resume("ask", pipeline.Result{Cancelled: true}); err != nil {
						t.Errorf("resume: %v", err)
					}
				}
			})
			if tt.wantFailed != (err != nil) {
				t.Fatalf("run error = %v, want error=%v", err, tt.wantFailed)
			}
			if tt.wantFailed && !errors.Is(err, pipeline.ErrCancelled) {
				t.Fatalf("run error = %v, want ErrCancelled", err)
			}
			ev, ok := findEvent(evs, "ask", tt.wantKind)
			if !ok || !errors.Is(ev.Err, pipeline.ErrCancelled) {
				t.Fatalf("event = %+v ok=%v, want kind %v with ErrCancelled", ev, ok, tt.wantKind)
			}
			if last := lastEvent(t, evs); last.Failed != tt.wantFailed {
				t.Fatalf("pipeline done failed = %v, want %v", last.Failed, tt.wantFailed)
			}
		})
	}
}

func TestResumeNotWaiting(t *testing.T) {
	step := waitingStep("ask", pipeline.Meta{Kind: pipeline.Interactive}, nil)
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Resume("ask", pipeline.Result{}); err == nil {
		t.Fatal("resume before run succeeded")
	}
	_, err = runPipeline(t, context.Background(), p, func(ev pipeline.Event) {
		if ev.Kind != pipeline.EvWaiting {
			return
		}
		if err := p.Resume("other", pipeline.Result{}); err == nil {
			t.Error("resume of unknown step succeeded")
		}
		if err := p.Resume("ask", pipeline.Result{Value: "v"}); err != nil {
			t.Errorf("resume: %v", err)
		}
		if err := p.Resume("ask", pipeline.Result{}); err == nil {
			t.Error("second resume succeeded")
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := p.Resume("ask", pipeline.Result{}); err == nil {
		t.Fatal("resume after pipeline done succeeded")
	}
}

func TestResumeOnRunningAutoStep(t *testing.T) {
	release := make(chan struct{})
	step := &fakeStep{
		meta: pipeline.Meta{Name: "auto"},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			<-release
			return nil
		},
	}
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runPipeline(t, context.Background(), p, func(ev pipeline.Event) {
		if ev.Kind == pipeline.EvStepStarted {
			if err := p.Resume("auto", pipeline.Result{}); err == nil {
				t.Error("resume of running non-waiting step succeeded")
			}
			close(release)
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestTimeoutStuckAutoStep(t *testing.T) {
	step := &fakeStep{
		meta: pipeline.Meta{Name: "stuck", Timeout: 50 * time.Millisecond},
		runFn: func(ctx context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run error = %v, want deadline exceeded", err)
	}
	failed, ok := findEvent(evs, "stuck", pipeline.EvFailed)
	if !ok || !errors.Is(failed.Err, context.DeadlineExceeded) {
		t.Fatalf("failed = %+v ok=%v, want EvFailed with deadline exceeded", failed, ok)
	}
}

func TestTimeoutSuspendedWhileWaiting(t *testing.T) {
	var got atomic.Value
	step := waitingStep("ask", pipeline.Meta{Kind: pipeline.Interactive, Timeout: 150 * time.Millisecond}, &got)
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, func(ev pipeline.Event) {
		if ev.Kind == pipeline.EvWaiting {
			time.Sleep(500 * time.Millisecond)
			if err := p.Resume("ask", pipeline.Result{Value: "late"}); err != nil {
				t.Errorf("resume: %v", err)
			}
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got.Load() != "late" {
		t.Fatalf("step got %v, want late", got.Load())
	}
	if _, ok := findEvent(evs, "ask", pipeline.EvFailed); ok {
		t.Fatal("timeout fired while step awaited resume")
	}
	if _, ok := findEvent(evs, "ask", pipeline.EvDone); !ok {
		t.Fatal("no EvDone for ask")
	}
}

func TestRetryEventualSuccess(t *testing.T) {
	var runs atomic.Int32
	step := &fakeStep{
		meta: pipeline.Meta{Name: "flaky", Retry: pipeline.RetryPolicy{Attempts: 3, Delay: time.Millisecond}},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			if runs.Add(1) < 3 {
				return errors.New("flaky")
			}
			return nil
		},
	}
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if runs.Load() != 3 {
		t.Fatalf("runs = %d, want 3", runs.Load())
	}
	if _, ok := findEvent(evs, "flaky", pipeline.EvDone); !ok {
		t.Fatal("no EvDone for flaky")
	}
	var started int
	for _, ev := range evs {
		if ev.Step == "flaky" && ev.Kind == pipeline.EvStepStarted {
			started++
		}
	}
	if started != 1 {
		t.Fatalf("EvStepStarted count = %d, want 1", started)
	}
}

func TestRetryRecheckShortCircuit(t *testing.T) {
	var runs atomic.Int32
	var ran atomic.Bool
	step := &fakeStep{
		meta:    pipeline.Meta{Name: "partial", Retry: pipeline.RetryPolicy{Attempts: 3, Delay: time.Millisecond}},
		checkFn: func(context.Context) (bool, error) { return ran.Load(), nil },
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			runs.Add(1)
			ran.Store(true)
			return errors.New("partial")
		},
	}
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if runs.Load() != 1 {
		t.Fatalf("runs = %d, want 1", runs.Load())
	}
	if _, ok := findEvent(evs, "partial", pipeline.EvDone); !ok {
		t.Fatal("no EvDone for partial")
	}
}

func TestRetryExhaustion(t *testing.T) {
	var runs atomic.Int32
	errBoom := errors.New("boom")
	step := &fakeStep{
		meta: pipeline.Meta{Name: "doomed", Retry: pipeline.RetryPolicy{Attempts: 2, Delay: time.Millisecond}},
		runFn: func(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
			runs.Add(1)
			return errBoom
		},
	}
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if !errors.Is(err, errBoom) {
		t.Fatalf("run error = %v, want boom", err)
	}
	if runs.Load() != 2 {
		t.Fatalf("runs = %d, want 2", runs.Load())
	}
	failed, ok := findEvent(evs, "doomed", pipeline.EvFailed)
	if !ok || !errors.Is(failed.Err, errBoom) {
		t.Fatalf("failed = %+v ok=%v, want EvFailed with boom", failed, ok)
	}
}

func TestCtxCancelMidRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rec := &recorder{}
	stuck := &fakeStep{
		meta: pipeline.Meta{Name: "stuck"},
		runFn: func(ctx context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	p, err := pipeline.New(nil, stuck, autoStep("after", rec))
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, ctx, p, func(ev pipeline.Event) {
		if ev.Kind == pipeline.EvStepStarted && ev.Step == "stuck" {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v, want canceled", err)
	}
	if _, ok := findEvent(evs, "stuck", pipeline.EvFailed); !ok {
		t.Fatal("no EvFailed for stuck")
	}
	if kinds := stepKinds(evs, "after"); len(kinds) != 0 {
		t.Fatalf("after events = %v, want none", kinds)
	}
	if got := rec.names(); len(got) != 0 {
		t.Fatalf("ran %v, want none", got)
	}
	if last := lastEvent(t, evs); last.Kind != pipeline.EvPipelineDone || !last.Failed {
		t.Fatalf("last event = %+v, want EvPipelineDone Failed=true", last)
	}
}

func TestCtxCancelMidWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var gotCancelled atomic.Bool
	step := &fakeStep{
		meta: pipeline.Meta{Name: "ask", Kind: pipeline.Interactive},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvWaiting, Payload: "prompt"}
			r := <-in
			if r.Cancelled {
				gotCancelled.Store(true)
				return pipeline.ErrCancelled
			}
			return nil
		},
	}
	p, err := pipeline.New(nil, step)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, ctx, p, func(ev pipeline.Event) {
		if ev.Kind == pipeline.EvWaiting {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v, want canceled", err)
	}
	if !gotCancelled.Load() {
		t.Fatal("step did not receive Result{Cancelled: true}")
	}
	failed, ok := findEvent(evs, "ask", pipeline.EvFailed)
	if !ok || !errors.Is(failed.Err, pipeline.ErrCancelled) {
		t.Fatalf("failed = %+v ok=%v, want EvFailed with ErrCancelled", failed, ok)
	}
	if last := lastEvent(t, evs); last.Kind != pipeline.EvPipelineDone || !last.Failed {
		t.Fatalf("last event = %+v, want EvPipelineDone Failed=true", last)
	}
}

func TestRunReturnsWhenConsumerStopsDrainingAndCtxCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flooding := &fakeStep{
		meta: pipeline.Meta{Name: "flood"},
		runFn: func(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
			for i := 0; i < 1000; i++ {
				select {
				case out <- pipeline.Event{Kind: pipeline.EvLine, Line: "x"}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		},
	}
	p, err := pipeline.New(nil, flooding)
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Run hung after ctx cancel with consumer not draining events")
	}
}

func TestRunTwice(t *testing.T) {
	rec := &recorder{}
	p, err := pipeline.New(nil, autoStep("a", rec))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runPipeline(t, context.Background(), p, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := p.Run(context.Background()); err == nil {
		t.Fatal("second run succeeded")
	}
}

func TestEventsChannelCloses(t *testing.T) {
	rec := &recorder{}
	p, err := pipeline.New(nil, autoStep("a", rec))
	if err != nil {
		t.Fatal(err)
	}
	evs, err := runPipeline(t, context.Background(), p, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if last := lastEvent(t, evs); last.Kind != pipeline.EvPipelineDone {
		t.Fatalf("last event = %+v, want EvPipelineDone", last)
	}
	if _, open := <-p.Events(); open {
		t.Fatal("events channel still open after EvPipelineDone")
	}
}

func TestNoGoroutineLeaks(t *testing.T) {
	before := runtime.NumGoroutine()

	t.Run("cancel mid run", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		step := &fakeStep{
			meta: pipeline.Meta{Name: "stuck"},
			runFn: func(ctx context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
				<-ctx.Done()
				return ctx.Err()
			},
		}
		p, err := pipeline.New(nil, step)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = runPipeline(t, ctx, p, func(ev pipeline.Event) {
			if ev.Kind == pipeline.EvStepStarted {
				cancel()
			}
		})
	})

	t.Run("cancel mid wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		step := waitingStep("ask", pipeline.Meta{Kind: pipeline.Interactive, Timeout: time.Second}, nil)
		p, err := pipeline.New(nil, step)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = runPipeline(t, ctx, p, func(ev pipeline.Event) {
			if ev.Kind == pipeline.EvWaiting {
				cancel()
			}
		})
	})

	t.Run("timeout", func(t *testing.T) {
		step := &fakeStep{
			meta: pipeline.Meta{Name: "slow", Timeout: 20 * time.Millisecond},
			runFn: func(ctx context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
				<-ctx.Done()
				return ctx.Err()
			},
		}
		p, err := pipeline.New(nil, step)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = runPipeline(t, context.Background(), p, nil)
	})

	t.Run("normal", func(t *testing.T) {
		rec := &recorder{}
		p, err := pipeline.New(nil, autoStep("a", rec), autoStep("b", rec, "a"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runPipeline(t, context.Background(), p, nil); err != nil {
			t.Fatalf("run: %v", err)
		}
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	t.Fatalf("goroutines: %d > %d before\n%s", runtime.NumGoroutine(), before, buf[:n])
}
