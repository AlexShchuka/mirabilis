package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/ui"
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

func TestPipelineInit_NonNilCmd(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
	})
	cmd := p.Init()
	if cmd == nil {
		t.Error("Init() returned nil cmd, want non-nil batch")
	}
	if p.start.IsZero() {
		t.Error("Init() did not set start time")
	}
}

func TestPipelineUpdate_WindowSizeMsg(t *testing.T) {
	p := trivialPipeline(nil)
	p2, _ := p.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if p2.progress.Width() != progressWidth(80) {
		t.Errorf("progress width after WindowSizeMsg = %d, want %d", p2.progress.Width(), progressWidth(80))
	}
}

func TestPipelineUpdate_SpinnerTickMsg(t *testing.T) {
	p := trivialPipeline(nil)
	p2, cmd := p.Update(spinner.TickMsg{})
	if p2 == nil {
		t.Fatal("Update returned nil pipeline")
	}
	_ = cmd
}

func TestPipelineUpdate_ProgressFrameMsg(t *testing.T) {
	p := trivialPipeline(nil)
	p2, cmd := p.Update(progress.FrameMsg{})
	if p2 == nil {
		t.Fatal("Update returned nil pipeline")
	}
	_ = cmd
}

func TestPipelineUpdate_UnknownMsg(t *testing.T) {
	p := trivialPipeline(nil)
	p2, cmd := p.Update("unknown msg")
	if p2 == nil {
		t.Fatal("Update returned nil")
	}
	if cmd != nil {
		cmd()
	}
}

func TestPipelineView_RunningStep(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a", Title: "doing-a"}, funcStep{}),
	})
	p.views[0].status = stRunning
	v := p.View()
	if !strings.Contains(v, ui.PipelineTitle) {
		t.Errorf("View missing PipelineTitle, got:\n%s", v)
	}
	if !strings.Contains(v, "doing-a") {
		t.Errorf("View missing running step title, got:\n%s", v)
	}
	if !strings.Contains(v, ui.HintEscCancel) {
		t.Errorf("View missing HintEscCancel, got:\n%s", v)
	}
}

func TestPipelineView_FailedStep(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a", Title: "bad-step"}, funcStep{}),
	})
	p.views[0].status = stFailed
	p.views[0].err = errBoom
	p.failed = true
	v := p.View()
	if !strings.Contains(v, "bad-step") {
		t.Errorf("View missing failed step title, got:\n%s", v)
	}
	if !strings.Contains(v, ui.HintAnyKeyMenu) {
		t.Errorf("View missing HintAnyKeyMenu, got:\n%s", v)
	}
}

func TestPipelineView_AllDone(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a", Title: "step-a"}, funcStep{}),
	})
	p.views[0].status = stDone
	v := p.View()
	if !strings.Contains(v, ui.LabelDone) {
		t.Errorf("View missing LabelDone, got:\n%s", v)
	}
}

func TestPipelineElapsed(t *testing.T) {
	p := trivialPipeline(nil)
	if p.elapsed() != 0 {
		t.Error("elapsed with zero start should be 0")
	}
	p.start = time.Now().Add(-5 * time.Second)
	if p.elapsed() < 4*time.Second {
		t.Error("elapsed should be >= 4s after setting start to 5s ago")
	}
}

func TestPipelineFmtElapsed_65s(t *testing.T) {
	got := FmtElapsed(65 * time.Second)
	if got != "01:05" {
		t.Errorf("FmtElapsed(65s) = %q, want 01:05", got)
	}
}

func TestPipelineSetProgress_EmptyViews(t *testing.T) {
	p := trivialPipeline(nil)
	cmd := p.setProgress()
	if cmd != nil {
		t.Error("setProgress with no views should return nil cmd")
	}
}

func TestPipelineSetProgress_NonEmpty(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
	})
	cmd := p.setProgress()
	if cmd == nil {
		t.Error("setProgress with views should return non-nil cmd")
	}
}

func TestPipelineUpdate_CheckedMsg(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
	})
	p.views[0].status = stRunning
	p2, cmd := p.Update(CheckedMsg{Name: "a", Satisfied: true})
	if p2 == nil {
		t.Fatal("Update returned nil")
	}
	if p2.views[0].status != stSkipped {
		t.Errorf("status after CheckedMsg(satisfied) = %v, want skipped", p2.views[0].status)
	}
	_ = cmd
}

func TestPipelineUpdate_RanMsg(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
	})
	p.views[0].status = stRunning
	p2, _ := p.Update(RanMsg{Name: "a", Err: nil})
	if p2.views[0].status != stDone {
		t.Errorf("status after RanMsg(ok) = %v, want done", p2.views[0].status)
	}
}

func TestPipelineView_CurrentDetail(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a", Title: "step-a", Detail: "doing detail"}, funcStep{}),
	})
	p.views[0].status = stRunning
	v := p.View()
	if !strings.Contains(v, "doing detail") {
		t.Errorf("View missing currentDetail, got:\n%s", v)
	}
}

func TestPipelineOnRan_Interactive_Retick(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "gh", Interactive: true, Optional: true}, funcStep{}),
	})
	p.views[0].status = stRunning
	p.interacting = true
	cmd := p.onRan(RanMsg{Name: "gh", Err: nil})
	if p.interacting {
		t.Error("onRan should clear interacting for interactive step")
	}
	if cmd == nil {
		t.Error("onRan interactive step should return non-nil batch cmd (re-tick)")
	}
}

func TestCascadeSkip_RequiredFailMakesDependentSkipped(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
		reg(StepMeta{Name: "b", Deps: []string{"a"}}, funcStep{}),
	})
	p.views[0].status = stRunning
	p.onChecked(CheckedMsg{Name: "a", Err: errBoom})
	if p.views[0].status != stFailed {
		t.Errorf("a status = %v, want stFailed", p.views[0].status)
	}
	if p.views[1].status != stSkipped {
		t.Errorf("b status = %v, want stSkipped (cascaded)", p.views[1].status)
	}
	if !p.failed {
		t.Error("pipeline failed flag not set")
	}
}

func TestCascadeSkip_RequiredRunFailMakesDependentSkipped(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
		reg(StepMeta{Name: "b", Deps: []string{"a"}}, funcStep{}),
	})
	p.views[0].status = stRunning
	p.onRan(RanMsg{Name: "a", Err: errBoom})
	if p.views[0].status != stFailed {
		t.Errorf("a status = %v, want stFailed", p.views[0].status)
	}
	if p.views[1].status != stSkipped {
		t.Errorf("b status = %v, want stSkipped (cascaded)", p.views[1].status)
	}
}

func TestCascadeSkip_DoneEmitted(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a"}, funcStep{}),
		reg(StepMeta{Name: "b", Deps: []string{"a"}}, funcStep{}),
	})
	p.views[0].status = stRunning
	p.onRan(RanMsg{Name: "a", Err: errBoom})
	if !p.done() {
		t.Error("pipeline should be done after required-step failure cascades all dependents to skipped")
	}
	if !p.failed {
		t.Error("pipeline failed flag not set, DoneMsg{Failed:true} would not be emitted")
	}
}

func TestCascadeSkip_OptionalFailDoesNotCascade(t *testing.T) {
	p := trivialPipeline([]Registered{
		reg(StepMeta{Name: "a", Optional: true}, funcStep{}),
		reg(StepMeta{Name: "b", Deps: []string{"a"}}, funcStep{}),
	})
	p.views[0].status = stRunning
	p.onRan(RanMsg{Name: "a", Err: errBoom})
	if p.views[0].status != stSkipped {
		t.Errorf("optional a status = %v, want stSkipped", p.views[0].status)
	}
	if p.views[1].status == stSkipped {
		t.Error("b should not be cascade-skipped when a is optional")
	}
}
