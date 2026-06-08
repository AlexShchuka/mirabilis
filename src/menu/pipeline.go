package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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
	Name     string
	Title    string
	Deps     []string
	Retry    RetryPolicy
	Optional bool

	Check   func(ctx context.Context, r Runner) (bool, error)
	Run     func(ctx context.Context, r Runner) error
	ExecCmd func(r Runner) *exec.Cmd
}

func (s Step) interactive() bool { return s.ExecCmd != nil }

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
	ctx    context.Context
	r      Runner
	views  []*stepView
	byName map[string]*stepView

	queue       []*stepView
	interacting bool
	failed      bool
}

func newPipeline(ctx context.Context, r Runner, steps []Step) *pipeline {
	p := &pipeline{ctx: ctx, r: r, byName: map[string]*stepView{}}
	for _, s := range steps {
		v := &stepView{step: s, status: stPending}
		p.views = append(p.views, v)
		p.byName[s.Name] = v
	}
	return p
}

func (p *pipeline) Init() tea.Cmd { return p.advance() }

func (p *pipeline) advance() tea.Cmd {
	var cmds []tea.Cmd
	for _, v := range p.views {
		if v.status != stPending || !p.depsReady(v.step) {
			continue
		}
		v.status = stRunning
		cmds = append(cmds, p.checkCmd(v.step))
	}
	if cmd := p.drainInteractive(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	if len(cmds) == 0 && !p.inFlight() {
		return tea.Quit
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

func (p *pipeline) inFlight() bool {
	if p.interacting || len(p.queue) > 0 {
		return true
	}
	for _, v := range p.views {
		if v.status == stRunning {
			return true
		}
	}
	return false
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

func (p *pipeline) execCmd(s Step) tea.Cmd {
	cmd := s.ExecCmd(p.r)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return ranMsg{name: s.Name, err: err}
	})
}

func (p *pipeline) drainInteractive() tea.Cmd {
	if p.interacting || len(p.queue) == 0 {
		return nil
	}
	v := p.queue[0]
	p.queue = p.queue[1:]
	p.interacting = true
	v.status = stRunning
	return p.execCmd(v.step)
}

func (p *pipeline) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case checkedMsg:
		v := p.byName[m.name]
		switch {
		case m.err != nil && v.step.Optional:
			v.status, v.err = stSkipped, m.err
		case m.err != nil:
			v.status, v.err = stFailed, m.err
			p.failed = true
		case m.satisfied:
			v.status = stSkipped
		case v.step.interactive():
			v.status = stWaiting
			p.queue = append(p.queue, v)
		default:
			return p, tea.Batch(p.runCmd(v.step), p.advance())
		}
		return p, p.advance()

	case ranMsg:
		v := p.byName[m.name]
		if v.step.interactive() {
			p.interacting = false
		}
		switch {
		case m.err == nil:
			v.status = stDone
		case v.step.Optional:
			v.status, v.err = stSkipped, m.err
		default:
			v.status, v.err = stFailed, m.err
			p.failed = true
		}
		return p, p.advance()

	case tea.KeyPressMsg:
		if s := m.String(); s == "ctrl+c" {
			p.failed = true
			return p, tea.Quit
		}
	}
	return p, nil
}

var (
	okMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failMark = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	dimMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (p *pipeline) View() tea.View {
	var b strings.Builder
	b.WriteString(dimMark.Render("mirabilis — launch") + "\n\n")
	for _, v := range p.views {
		var glyph string
		switch v.status {
		case stDone:
			glyph = okMark.Render("✔")
		case stSkipped:
			glyph = dimMark.Render("•")
		case stFailed:
			glyph = failMark.Render("✘")
		case stRunning:
			glyph = "▸"
		case stWaiting:
			glyph = dimMark.Render("…")
		default:
			glyph = dimMark.Render(" ")
		}
		line := fmt.Sprintf(" %s %s", glyph, v.step.Title)
		if v.err != nil {
			line += dimMark.Render(" — " + v.err.Error())
		}
		b.WriteString(line + "\n")
	}
	return tea.NewView(b.String())
}
