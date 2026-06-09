package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

var errBoom = errors.New("boom")

type funcStep struct {
	check func(ctx context.Context, r runner.Runner) (bool, error)
	run   func(ctx context.Context, r runner.Runner) error
}

func (s funcStep) Check(ctx context.Context, r runner.Runner) (bool, error) {
	if s.check == nil {
		return false, nil
	}
	return s.check(ctx, r)
}

func (s funcStep) Run(ctx context.Context, r runner.Runner) error {
	if s.run == nil {
		return nil
	}
	return s.run(ctx, r)
}

func trivialPipeline(regs []Registered) *Pipeline {
	return NewPipeline(context.Background(), &runner.FakeRunner{}, regs)
}

func reg(meta StepMeta, impl Step) Registered { return Registered{Meta: meta, Impl: impl} }

func TestStepCheckTimeout(t *testing.T) {
	r := reg(StepMeta{Name: "slow", Timeout: 30 * time.Millisecond}, funcStep{
		check: func(ctx context.Context, _ runner.Runner) (bool, error) {
			<-ctx.Done()
			return false, ctx.Err()
		},
	})
	p := trivialPipeline([]Registered{r})
	msg, ok := p.checkCmd(&r)().(CheckedMsg)
	if !ok {
		t.Fatal("checkCmd did not return a CheckedMsg")
	}
	if !errors.Is(msg.Err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", msg.Err)
	}
}

func TestStepRunTimeout(t *testing.T) {
	r := reg(StepMeta{Name: "slow", Retry: RetryNone, Timeout: 30 * time.Millisecond}, funcStep{
		run: func(ctx context.Context, _ runner.Runner) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	p := trivialPipeline([]Registered{r})
	msg, ok := p.runCmd(&r)().(RanMsg)
	if !ok {
		t.Fatal("runCmd did not return a RanMsg")
	}
	if !errors.Is(msg.Err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", msg.Err)
	}
}

func TestStepNoTimeoutDoesNotCancel(t *testing.T) {
	r := reg(StepMeta{Name: "quick"}, funcStep{
		check: func(ctx context.Context, _ runner.Runner) (bool, error) {
			return ctx.Err() == nil, nil
		},
	})
	p := trivialPipeline([]Registered{r})
	msg := p.checkCmd(&r)().(CheckedMsg)
	if !msg.Satisfied {
		t.Error("a step with no Timeout must run with a live (uncancelled) context")
	}
}

func TestPipelineCurrentDetail(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a", Detail: "doing a"}, funcStep{}),
		reg(StepMeta{Name: "b", Detail: "doing b"}, funcStep{}),
	})
	if got := p.currentDetail(); got != "" {
		t.Errorf("currentDetail with nothing running = %q, want empty", got)
	}
	p.views[0].status = stDone
	p.views[1].status = stRunning
	if got := p.currentDetail(); got != "doing b" {
		t.Errorf("currentDetail = %q, want %q", got, "doing b")
	}
}

func TestFmtElapsed(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00"},
		{5 * time.Second, "00:05"},
		{83 * time.Second, "01:23"},
		{600 * time.Second, "10:00"},
	}
	for _, tt := range tests {
		if got := FmtElapsed(tt.d); got != tt.want {
			t.Errorf("FmtElapsed(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestPipelineOnChecked(t *testing.T) {
	tests := []struct {
		give       CheckedMsg
		name       string
		wantStatus stepStatus
		optional   bool
		wantFailed bool
	}{
		{name: "satisfied -> skipped", give: CheckedMsg{Name: "s", Satisfied: true}, wantStatus: stSkipped},
		{name: "needs run -> running", give: CheckedMsg{Name: "s", Satisfied: false}, wantStatus: stRunning},
		{name: "error non-optional -> failed", give: CheckedMsg{Name: "s", Err: errBoom}, wantStatus: stFailed, wantFailed: true},
		{name: "error optional -> skipped", optional: true, give: CheckedMsg{Name: "s", Err: errBoom}, wantStatus: stSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := trivialPipeline([]Registered{reg(StepMeta{Name: "s", Optional: tt.optional}, funcStep{})})
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
		err        error
		name       string
		wantStatus stepStatus
		optional   bool
		wantFailed bool
	}{
		{name: "success -> done", wantStatus: stDone},
		{name: "error optional -> skipped", optional: true, err: errBoom, wantStatus: stSkipped},
		{name: "error non-optional -> failed", err: errBoom, wantStatus: stFailed, wantFailed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := trivialPipeline([]Registered{reg(StepMeta{Name: "s", Optional: tt.optional}, funcStep{})})
			p.views[0].status = stRunning
			p.onRan(RanMsg{Name: "s", Err: tt.err})
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
	p := trivialPipeline([]Registered{reg(StepMeta{Name: "gh", Interactive: true, Optional: true}, funcStep{})})
	p.queue = append(p.queue, p.views[0])

	cmd := p.drainInteractive()
	if cmd == nil {
		t.Fatal("drainInteractive returned nil with a queued step")
	}
	msg, ok := cmd().(NeedGHMsg)
	if !ok {
		t.Fatalf("emitted %T, want NeedGHMsg", cmd())
	}
	if msg.Name != "gh" {
		t.Errorf("NeedGH name = %q, want gh", msg.Name)
	}
	if !p.interacting {
		t.Error("drainInteractive should mark the pipeline interacting")
	}
}

func TestPipelineInteractiveStepGoesInteracting(t *testing.T) {
	p := trivialPipeline([]Registered{reg(StepMeta{Name: "gh", Interactive: true, Optional: true}, funcStep{})})
	p.views[0].status = stRunning
	p.onChecked(CheckedMsg{Name: "gh", Satisfied: false})
	if !p.interacting {
		t.Error("an interactive step should drive the pipeline interacting after its check")
	}
}

func TestPipelineDone(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
		reg(StepMeta{Name: "b"}, funcStep{}),
	})
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
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
		reg(StepMeta{Name: "b"}, funcStep{}),
		reg(StepMeta{Name: "c"}, funcStep{}),
	})
	p.views[0].status = stDone
	p.views[1].status = stFailed
	p.views[2].status = stRunning
	if got := p.resolved(); got != 2 {
		t.Errorf("resolved = %d, want 2", got)
	}
}

func TestPipelineDepsReady(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
		reg(StepMeta{Name: "b", Deps: []string{"a"}}, funcStep{}),
	})
	if p.depsReady(p.views[1].reg.Meta) {
		t.Error("b should not be ready while a is pending")
	}
	p.views[0].status = stDone
	if !p.depsReady(p.views[1].reg.Meta) {
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
