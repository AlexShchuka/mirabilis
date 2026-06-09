package main

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

type phase int

const (
	phaseMenu phase = iota
	phasePipeline
	phaseForm
	phaseGHAuth
)

type menuChoiceMsg struct{ action string }
type backToMenuMsg struct{ notice string }
type pipelineDoneMsg struct{ failed bool }
type needGHMsg struct{ name string }
type ghDoneMsg struct{ err error }

func emit(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

type appModel struct {
	ctx context.Context
	r   Runner

	phase  phase
	w, h   int
	notice string

	menu    menuModel
	pipe    *pipeline
	form    *formScreen
	gh      *ghAuthModel
	pendGH  string
	pipeEnd bool

	handoff bool
}

var _ tea.Model = appModel{}

func newApp(ctx context.Context, r Runner, st Status) appModel {
	return appModel{ctx: ctx, r: r, phase: phaseMenu, menu: newMenu(st)}
}

func (a appModel) Init() tea.Cmd { return a.menu.Init() }

func (a appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "ctrl+c" {
		return a, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
		return a.forwardSize(msg)
	case menuChoiceMsg:
		return a.route(msg.action)
	case backToMenuMsg:
		return a.toMenu(msg.notice)
	case pipelineDoneMsg:
		if msg.failed {
			a.pipeEnd = true
			return a, nil
		}
		a.handoff = true
		return a, tea.Quit
	case needGHMsg:
		a.phase = phaseGHAuth
		a.pendGH = msg.name
		a.gh = newGHAuth(a.ctx, a.r, a.w, a.h)
		return a, a.gh.Init()
	case ghDoneMsg:
		if a.pipe == nil {
			return a.toMenu("")
		}
		a.phase = phasePipeline
		var cmd tea.Cmd
		a.pipe, cmd = a.pipe.Update(ranMsg{name: a.pendGH, err: msg.err})
		return a, cmd
	}

	switch a.phase {
	case phaseMenu:
		var cmd tea.Cmd
		a.menu, cmd = a.menu.Update(msg)
		return a, cmd
	case phasePipeline:
		if a.pipeEnd {
			if _, ok := msg.(tea.KeyPressMsg); ok {
				return a.toMenu("")
			}
			return a, nil
		}
		var cmd tea.Cmd
		a.pipe, cmd = a.pipe.Update(msg)
		return a, cmd
	case phaseForm:
		return a.updateForm(msg)
	case phaseGHAuth:
		var cmd tea.Cmd
		a.gh, cmd = a.gh.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a appModel) route(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "launch":
		a.phase = phasePipeline
		a.pipeEnd = false
		a.pipe = newPipeline(a.ctx, a.r, buildSteps())
		return a, tea.Batch(sizeCmd(a.w, a.h), a.pipe.Init())
	case "plugins":
		f := newPluginsForm(a.ctx, a.r, a.w, a.h)
		if f == nil {
			return a.toMenu("плагины: каталог недоступен")
		}
		a.form, a.phase = f, phaseForm
		return a, f.Init()
	case "harness":
		a.form, a.phase = newHarnessForm(a.ctx, a.r, a.w, a.h), phaseForm
		return a, a.form.Init()
	case "stacks":
		f := newStacksForm(a.r, a.w, a.h)
		if f == nil {
			return a.toMenu("стеки: каталог недоступен")
		}
		a.form, a.phase = f, phaseForm
		return a, f.Init()
	case "vscode":
		return a, doVSCodeCmd(a.ctx, a.r)
	case "quit":
		return a, tea.Quit
	}
	return a, nil
}

func (a appModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.form, cmd = a.form.Update(msg)
	switch {
	case a.form.completed():
		return a, a.form.apply()
	case a.form.aborted():
		return a, emit(backToMenuMsg{})
	}
	return a, cmd
}

func (a appModel) toMenu(notice string) (tea.Model, tea.Cmd) {
	a.phase = phaseMenu
	a.pipe, a.form, a.gh = nil, nil, nil
	a.pipeEnd = false
	a.notice = notice
	a.menu = newMenu(computeStatus(a.ctx, a.r))
	var cmd tea.Cmd
	a.menu, cmd = a.menu.Update(tea.WindowSizeMsg{Width: a.w, Height: a.h})
	return a, cmd
}

func (a appModel) forwardSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.phase {
	case phaseMenu:
		a.menu, cmd = a.menu.Update(msg)
	case phasePipeline:
		if a.pipe != nil {
			a.pipe, cmd = a.pipe.Update(msg)
		}
	case phaseForm:
		if a.form != nil {
			a.form, cmd = a.form.Update(msg)
		}
	case phaseGHAuth:
		if a.gh != nil {
			a.gh, cmd = a.gh.Update(msg)
		}
	}
	return a, cmd
}

func sizeCmd(w, h int) tea.Cmd {
	return emit(tea.WindowSizeMsg{Width: w, Height: h})
}

func (a appModel) View() tea.View {
	var content string
	switch a.phase {
	case phaseMenu:
		content = a.menu.View()
		if a.notice != "" {
			content += "\n " + offStyle.Render(a.notice)
		}
	case phasePipeline:
		if a.pipe != nil {
			content = a.pipe.View()
		}
	case phaseForm:
		if a.form != nil {
			content = a.form.View()
		}
	case phaseGHAuth:
		if a.gh != nil {
			content = a.gh.View()
		}
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
