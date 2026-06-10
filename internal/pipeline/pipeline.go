package pipeline

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/ui"
)

type Step interface {
	Check(ctx context.Context, r runner.Runner) (bool, error)
	Run(ctx context.Context, r runner.Runner) error
}

type StepMeta struct {
	Name        string
	Title       string
	Detail      string
	Deps        []string
	Retry       RetryPolicy
	Optional    bool
	Interactive bool
	Timeout     time.Duration
}

type Registered struct {
	Impl Step
	Meta StepMeta
}

type stepStatus int

const (
	stPending stepStatus = iota
	stRunning
	stWaiting
	stSkipped
	stDone
	stFailed
)

type stepView struct {
	err    error
	reg    *Registered
	status stepStatus
}

type CheckedMsg struct {
	Err       error
	Name      string
	Satisfied bool
}

type RanMsg struct {
	Err  error
	Name string
}

type DoneMsg struct{ Failed bool }
type NeedGHMsg struct{ Name string }

func emit(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

type Pipeline struct {
	progress    progress.Model
	start       time.Time
	ctx         context.Context
	r           runner.Runner
	byName      map[string]*stepView
	views       []*stepView
	queue       []*stepView
	spin        spinner.Model
	interacting bool
	failed      bool
}

func NewPipeline(ctx context.Context, r runner.Runner, steps []Registered) *Pipeline {
	p := &Pipeline{
		ctx:    ctx,
		r:      r,
		byName: map[string]*stepView{},
		spin:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		progress: progress.New(
			progress.WithColors(lipgloss.Color("#29fcc3"), lipgloss.Color("#0bd4cd")),
			progress.WithoutPercentage(),
		),
	}
	p.progress.SetWidth(40)
	for i := range steps {
		v := &stepView{reg: &steps[i], status: stPending}
		p.views = append(p.views, v)
		p.byName[steps[i].Meta.Name] = v
	}
	return p
}

func (p *Pipeline) Init() tea.Cmd {
	p.start = time.Now()
	return tea.Batch(p.spin.Tick, p.advance())
}

func (p *Pipeline) advance() tea.Cmd {
	var cmds []tea.Cmd
	for _, v := range p.views {
		if v.status == stPending && p.depsReady(v.reg.Meta) {
			v.status = stRunning
			cmds = append(cmds, p.checkCmd(v.reg))
		}
	}
	if cmd := p.drainInteractive(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, p.setProgress())
	if p.done() {
		cmds = append(cmds, emit(DoneMsg{Failed: p.failed}))
	}
	return tea.Batch(cmds...)
}

func (p *Pipeline) depsReady(m StepMeta) bool {
	for _, d := range m.Deps {
		dep, ok := p.byName[d]
		if !ok || (dep.status != stDone && dep.status != stSkipped) {
			return false
		}
	}
	return true
}

func (p *Pipeline) done() bool {
	if p.interacting || len(p.queue) > 0 {
		return false
	}
	for _, v := range p.views {
		switch v.status {
		case stPending, stRunning, stWaiting:
			return false
		}
	}
	return true
}

func (p *Pipeline) resolved() int {
	n := 0
	for _, v := range p.views {
		switch v.status {
		case stDone, stSkipped, stFailed:
			n++
		}
	}
	return n
}

func (p *Pipeline) setProgress() tea.Cmd {
	if len(p.views) == 0 {
		return nil
	}
	return p.progress.SetPercent(float64(p.resolved()) / float64(len(p.views)))
}

func (p *Pipeline) stepCtx(m StepMeta) (context.Context, context.CancelFunc) {
	if m.Timeout > 0 {
		return context.WithTimeout(p.ctx, m.Timeout)
	}
	return context.WithCancel(p.ctx)
}

func (p *Pipeline) checkCmd(reg *Registered) tea.Cmd {
	return func() tea.Msg {
		if reg.Impl == nil {
			return CheckedMsg{Name: reg.Meta.Name, Satisfied: false}
		}
		ctx, cancel := p.stepCtx(reg.Meta)
		defer cancel()
		ok, err := reg.Impl.Check(ctx, p.r)
		return CheckedMsg{Name: reg.Meta.Name, Satisfied: ok, Err: err}
	}
}

func (p *Pipeline) runCmd(reg *Registered) tea.Cmd {
	return func() tea.Msg {
		err := retry(p.ctx, reg.Meta.Retry, func() error {
			ctx, cancel := p.stepCtx(reg.Meta)
			defer cancel()
			return reg.Impl.Run(ctx, p.r)
		})
		return RanMsg{Name: reg.Meta.Name, Err: err}
	}
}

func (p *Pipeline) drainInteractive() tea.Cmd {
	if p.interacting || len(p.queue) == 0 {
		return nil
	}
	v := p.queue[0]
	p.queue = p.queue[1:]
	p.interacting = true
	v.status = stRunning
	return emit(NeedGHMsg{Name: v.reg.Meta.Name})
}

func (p *Pipeline) Update(msg tea.Msg) (*Pipeline, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.progress.SetWidth(progressWidth(m.Width))
		return p, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		p.spin, cmd = p.spin.Update(m)
		return p, cmd
	case progress.FrameMsg:
		var cmd tea.Cmd
		p.progress, cmd = p.progress.Update(m)
		return p, cmd
	case CheckedMsg:
		return p, p.onChecked(m)
	case RanMsg:
		return p, p.onRan(m)
	}
	return p, nil
}

func (p *Pipeline) cascadeSkip() {
	changed := true
	for changed {
		changed = false
		for _, v := range p.views {
			if v.status != stPending {
				continue
			}
			for _, d := range v.reg.Meta.Deps {
				dep, ok := p.byName[d]
				if ok && dep.status == stFailed && !dep.reg.Meta.Optional {
					v.status = stSkipped
					changed = true
					break
				}
			}
		}
	}
}

func (p *Pipeline) onChecked(m CheckedMsg) tea.Cmd {
	v, ok := p.byName[m.Name]
	if !ok {
		return p.advance()
	}
	switch {
	case m.Err != nil && v.reg.Meta.Optional:
		v.status = stSkipped
	case m.Err != nil:
		v.status, v.err = stFailed, m.Err
		p.failed = true
		p.cascadeSkip()
	case m.Satisfied:
		v.status = stSkipped
	case v.reg.Meta.Interactive:
		v.status = stWaiting
		p.queue = append(p.queue, v)
	default:
		return tea.Batch(p.runCmd(v.reg), p.advance())
	}
	return p.advance()
}

func (p *Pipeline) onRan(m RanMsg) tea.Cmd {
	v, ok := p.byName[m.Name]
	if !ok {
		return p.advance()
	}
	reTick := false
	if v.reg.Meta.Interactive {
		p.interacting = false
		reTick = true
	}
	switch {
	case m.Err == nil:
		v.status = stDone
	case v.reg.Meta.Optional:
		v.status = stSkipped
	default:
		v.status, v.err = stFailed, m.Err
		p.failed = true
		p.cascadeSkip()
	}
	if reTick {
		return tea.Batch(p.advance(), p.spin.Tick)
	}
	return p.advance()
}

func (p *Pipeline) View() string {
	label, failure := p.label()
	clock := ui.TitleStyle.Render(FmtElapsed(p.elapsed()))
	bar := p.spin.View() + " " + clock + " " + p.progress.View() + "  " + label
	out := ui.TitleStyle.Render(ui.PipelineTitle) + "\n\n" + bar
	if failure != "" {
		out += "\n\n" + ui.FailMarkStyle.Render("✘ ") + failure + "\n " + ui.HintStyle.Render(ui.HintAnyKeyMenu)
	} else {
		if d := p.currentDetail(); d != "" {
			out += "\n " + ui.HintStyle.Render(d)
		}
		out += "\n " + ui.HintStyle.Render(ui.HintEscCancel)
	}
	return out
}

func (p *Pipeline) currentDetail() string {
	for _, v := range p.views {
		if v.status == stRunning || v.status == stWaiting {
			return v.reg.Meta.Detail
		}
	}
	return ""
}

func (p *Pipeline) elapsed() time.Duration {
	if p.start.IsZero() {
		return 0
	}
	return time.Since(p.start)
}

func FmtElapsed(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func (p *Pipeline) label() (string, string) {
	for _, v := range p.views {
		if v.status == stFailed {
			msg := v.reg.Meta.Title
			if v.err != nil {
				msg += " — " + v.err.Error()
			}
			return v.reg.Meta.Title, msg
		}
	}
	for _, v := range p.views {
		if v.status == stRunning || v.status == stWaiting {
			return v.reg.Meta.Title + "…", ""
		}
	}
	return ui.LabelDone, ""
}

func progressWidth(w int) int {
	pw := w - 28
	if pw < 10 {
		return 10
	}
	if pw > 50 {
		return 50
	}
	return pw
}
