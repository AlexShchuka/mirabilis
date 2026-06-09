package main

import (
	"context"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type stepStatus int

const (
	stPending stepStatus = iota
	stRunning
	stWaiting
	stSkipped
	stDone
	stFailed
)

type Step struct {
	Name        string
	Title       string
	Deps        []string
	Retry       RetryPolicy
	Optional    bool
	Interactive bool

	Check func(ctx context.Context, r Runner) (bool, error)
	Run   func(ctx context.Context, r Runner) error
}

type stepView struct {
	step   Step
	status stepStatus
	err    error
}

type checkedMsg struct {
	name      string
	satisfied bool
	err       error
}

type ranMsg struct {
	name string
	err  error
}

type pipeline struct {
	ctx context.Context
	r   Runner

	views  []*stepView
	byName map[string]*stepView

	spin     spinner.Model
	progress progress.Model

	queue       []*stepView
	interacting bool
	failed      bool
}

func newPipeline(ctx context.Context, r Runner, steps []Step) *pipeline {
	p := &pipeline{
		ctx:      ctx,
		r:        r,
		byName:   map[string]*stepView{},
		spin:     spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		progress: progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage()),
	}
	p.progress.SetWidth(40)
	for _, s := range steps {
		v := &stepView{step: s, status: stPending}
		p.views = append(p.views, v)
		p.byName[s.Name] = v
	}
	return p
}

func (p *pipeline) Init() tea.Cmd {
	return tea.Batch(p.spin.Tick, p.advance())
}

func (p *pipeline) advance() tea.Cmd {
	var cmds []tea.Cmd
	for _, v := range p.views {
		if v.status == stPending && p.depsReady(v.step) {
			v.status = stRunning
			cmds = append(cmds, p.checkCmd(v.step))
		}
	}
	if cmd := p.drainInteractive(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, p.setProgress())
	if p.done() {
		cmds = append(cmds, emit(pipelineDoneMsg{failed: p.failed}))
	}
	return tea.Batch(cmds...)
}

func (p *pipeline) depsReady(s Step) bool {
	for _, d := range s.Deps {
		dep, ok := p.byName[d]
		if !ok || (dep.status != stDone && dep.status != stSkipped) {
			return false
		}
	}
	return true
}

func (p *pipeline) done() bool {
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

func (p *pipeline) resolved() int {
	n := 0
	for _, v := range p.views {
		switch v.status {
		case stDone, stSkipped, stFailed:
			n++
		}
	}
	return n
}

func (p *pipeline) setProgress() tea.Cmd {
	if len(p.views) == 0 {
		return nil
	}
	return p.progress.SetPercent(float64(p.resolved()) / float64(len(p.views)))
}

func (p *pipeline) checkCmd(s Step) tea.Cmd {
	return func() tea.Msg {
		if s.Check == nil {
			return checkedMsg{name: s.Name, satisfied: false}
		}
		ok, err := s.Check(p.ctx, p.r)
		return checkedMsg{name: s.Name, satisfied: ok, err: err}
	}
}

func (p *pipeline) runCmd(s Step) tea.Cmd {
	return func() tea.Msg {
		err := retry(p.ctx, s.Retry, func() error { return s.Run(p.ctx, p.r) })
		return ranMsg{name: s.Name, err: err}
	}
}

func (p *pipeline) drainInteractive() tea.Cmd {
	if p.interacting || len(p.queue) == 0 {
		return nil
	}
	v := p.queue[0]
	p.queue = p.queue[1:]
	p.interacting = true
	v.status = stRunning
	return emit(needGHMsg{name: v.step.Name})
}

func (p *pipeline) Update(msg tea.Msg) (*pipeline, tea.Cmd) {
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
	case checkedMsg:
		return p, p.onChecked(m)
	case ranMsg:
		return p, p.onRan(m)
	}
	return p, nil
}

func (p *pipeline) onChecked(m checkedMsg) tea.Cmd {
	v, ok := p.byName[m.name]
	if !ok {
		return p.advance()
	}
	switch {
	case m.err != nil && v.step.Optional:
		v.status = stSkipped
	case m.err != nil:
		v.status, v.err = stFailed, m.err
		p.failed = true
	case m.satisfied:
		v.status = stSkipped
	case v.step.Interactive:
		v.status = stWaiting
		p.queue = append(p.queue, v)
	default:
		return tea.Batch(p.runCmd(v.step), p.advance())
	}
	return p.advance()
}

func (p *pipeline) onRan(m ranMsg) tea.Cmd {
	v, ok := p.byName[m.name]
	if !ok {
		return p.advance()
	}
	reTick := false
	if v.step.Interactive {
		p.interacting = false
		reTick = true
	}
	switch {
	case m.err == nil:
		v.status = stDone
	case v.step.Optional:
		v.status = stSkipped
	default:
		v.status, v.err = stFailed, m.err
		p.failed = true
	}
	if reTick {
		return tea.Batch(p.advance(), p.spin.Tick)
	}
	return p.advance()
}

var (
	failMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (p *pipeline) View() string {
	label, failure := p.label()
	bar := p.spin.View() + " " + p.progress.View() + "  " + label
	out := titleStyle.Render("mirabilis — запуск") + "\n\n" + bar
	if failure != "" {
		out += "\n\n" + failMark.Render("✘ ") + failure + "\n " + hintStyle.Render("любая клавиша — в меню")
	}
	return out
}

func (p *pipeline) label() (string, string) {
	for _, v := range p.views {
		if v.status == stFailed {
			msg := v.step.Title
			if v.err != nil {
				msg += " — " + v.err.Error()
			}
			return v.step.Title, msg
		}
	}
	for _, v := range p.views {
		if v.status == stRunning || v.status == stWaiting {
			return v.step.Title + "…", ""
		}
	}
	return "готово", ""
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
