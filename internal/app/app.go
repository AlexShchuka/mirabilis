package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/ghauth"
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/steps"
	"github.com/AlexShchuka/mirabilis/internal/ui"
)

type appPhase int

const (
	phaseMenu appPhase = iota
	phasePipeline
	phaseForm
	phaseGHAuth
)

type menuChoiceMsg struct{ action string }
type backToMenuMsg struct{ notice string }
type telegramDoneMsg struct{}

func emit(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

type appModel struct {
	menu       menuModel
	r          runner.Runner
	ctx        context.Context
	form       *formScreen
	pipe       *pipeline.Pipeline
	pipeCancel context.CancelFunc
	gh         *ghauth.Model
	ghCancel   context.CancelFunc
	notice     string
	pendGH     string
	h          int
	w          int
	phase      appPhase
	pipeEnd    bool
	handoff    bool
}

var _ tea.Model = appModel{}

func newApp(ctx context.Context, r runner.Runner, st provision.Status) appModel {
	return appModel{ctx: ctx, r: r, phase: phaseMenu, menu: newMenu(st)}
}

func (a appModel) Init() tea.Cmd { return a.menu.Init() }

func (a appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "ctrl+c" {
		if a.ghCancel != nil {
			a.ghCancel()
			a.ghCancel = nil
		}
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
	case launchReadyMsg:
		f := newTelegramForm(a.w, a.h)
		a.form, a.phase = f, phaseForm
		return a, f.Init()
	case telegramDoneMsg:
		f := newClaudeTokenForm(a.ctx, a.r, a.w, a.h)
		a.form, a.phase = f, phaseForm
		return a, f.Init()
	case startPipelineMsg:
		a.form = nil
		return a.startPipeline()
	case pipeline.DoneMsg:
		if msg.Failed {
			a.pipeEnd = true
			return a, nil
		}
		a.handoff = true
		return a, tea.Quit
	case pipeline.NeedGHMsg:
		a.phase = phaseGHAuth
		a.pendGH = msg.Name
		ctx, cancel := context.WithCancel(a.ctx)
		a.ghCancel = cancel
		a.gh = ghauth.New(ctx, a.r, a.w, a.h)
		return a, a.gh.Init()
	case ghauth.DoneMsg:
		if a.ghCancel != nil {
			a.ghCancel()
			a.ghCancel = nil
		}
		if a.pipe == nil {
			return a.toMenu("")
		}
		a.phase = phasePipeline
		var cmd tea.Cmd
		a.pipe, cmd = a.pipe.Update(pipeline.RanMsg{Name: a.pendGH, Err: msg.Err})
		return a, cmd
	case pipeline.CheckedMsg:
		return a.forwardToPipe(msg)
	case pipeline.RanMsg:
		return a.forwardToPipe(msg)
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
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "esc" {
			return a.toMenu(ui.NoticeLaunchCanceled)
		}
		var cmd tea.Cmd
		a.pipe, cmd = a.pipe.Update(msg)
		return a, cmd
	case phaseForm:
		return a.updateForm(msg)
	case phaseGHAuth:
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "esc" {
			if a.ghCancel != nil {
				a.ghCancel()
				a.ghCancel = nil
			}
			return a.toMenu(ui.NoticeLaunchCanceled)
		}
		if a.gh == nil {
			return a, nil
		}
		var cmd tea.Cmd
		a.gh, cmd = a.gh.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a appModel) route(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "launch":
		f := newLaunchForm(a.r, a.w, a.h)
		if f == nil {
			tg := newTelegramForm(a.w, a.h)
			a.form, a.phase = tg, phaseForm
			return a, tg.Init()
		}
		a.form, a.phase = f, phaseForm
		return a, f.Init()
	case "harness":
		a.form, a.phase = newHarnessForm(a.ctx, a.r, a.w, a.h), phaseForm
		return a, a.form.Init()
	case "reset":
		a.form, a.phase = newResetForm(a.ctx, a.r, a.w, a.h), phaseForm
		return a, a.form.Init()
	case "vscode":
		return a, doVSCodeCmd(a.ctx, a.r)
	case "quit":
		return a, tea.Quit
	}
	return a, nil
}

func (a appModel) startPipeline() (tea.Model, tea.Cmd) {
	a.phase = phasePipeline
	a.pipeEnd = false
	ctx, cancel := context.WithCancel(a.ctx)
	a.pipeCancel = cancel
	a.pipe = pipeline.NewPipeline(ctx, a.r, steps.BuildSteps())
	return a, tea.Batch(sizeCmd(a.w, a.h), a.pipe.Init())
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
	if a.pipeCancel != nil {
		a.pipeCancel()
		a.pipeCancel = nil
	}
	if a.ghCancel != nil {
		a.ghCancel()
		a.ghCancel = nil
	}
	a.phase = phaseMenu
	a.pipe, a.form, a.gh = nil, nil, nil
	a.pipeEnd = false
	a.notice = notice
	a.menu = newMenu(provision.ComputeStatus(a.ctx, a.r))
	var cmd tea.Cmd
	a.menu, cmd = a.menu.Update(tea.WindowSizeMsg{Width: a.w, Height: a.h})
	return a, cmd
}

func (a appModel) forwardToPipe(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.pipe == nil {
		return a, nil
	}
	var cmd tea.Cmd
	a.pipe, cmd = a.pipe.Update(msg)
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
			content += "\n " + ui.OffStyle.Render(a.notice)
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
