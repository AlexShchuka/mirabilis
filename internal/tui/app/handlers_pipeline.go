package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/steps"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/form"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func (a App) handlePipelineEvent(msg pipelineEventMsg) (tea.Model, tea.Cmd) {
	ev := msg.ev
	cmds := []tea.Cmd{pumpEvents(a.pipe.Events())}

	switch ev.Kind {
	case pipeline.EvStepStarted:
		se := bus.StepEvent{Step: ev.Step, Kind: bus.StepStarted}
		a.router, _ = a.router.Update(bus.Envelope{Msg: se})

	case pipeline.EvSpawn:
		se := bus.StepEvent{Step: ev.Step, Kind: bus.StepStarted, Argv: ev.Argv}
		a.router, _ = a.router.Update(bus.Envelope{Msg: se})

	case pipeline.EvLine:
		se := bus.StepEvent{Step: ev.Step, Kind: bus.StepLine, Line: ev.Line}
		a.router, _ = a.router.Update(bus.Envelope{Msg: se})

	case pipeline.EvDone:
		se := bus.StepEvent{Step: ev.Step, Kind: bus.StepDone, Line: ev.Line}
		a.router, _ = a.router.Update(bus.Envelope{Msg: se})

	case pipeline.EvFailed:
		se := bus.StepEvent{Step: ev.Step, Kind: bus.StepFailed}
		if ev.Err != nil {
			se.Line = ev.Err.Error()
		}
		a.router, _ = a.router.Update(bus.Envelope{Msg: se})

	case pipeline.EvSkipped:
		se := bus.StepEvent{Step: ev.Step, Kind: bus.StepSkipped}
		a.router, _ = a.router.Update(bus.Envelope{Msg: se})

	case pipeline.EvWaiting:
		a.waiting = ev.Step
		wse := bus.StepEvent{Step: ev.Step, Kind: bus.StepWaiting}
		a.router, _ = a.router.Update(bus.Envelope{Msg: wse})
		c := a.handleWaiting(ev)
		cmds = append(cmds, c)

	case pipeline.EvPipelineDone:
		return a.handlePipelineDone(pipelineDoneMsg{failed: ev.Failed})
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleWaiting(ev pipeline.Event) tea.Cmd {
	if len(ev.Argv) > 0 {
		return a.handleTerminal(ev)
	}
	switch p := ev.Payload.(type) {
	case steps.Wizard:
		groups := make([]form.Group, 0, len(p.Groups))
		for _, c := range p.Groups {
			groups = append(groups, form.Group{
				Key:         c.Key,
				Title:       c.Title,
				Description: c.Description,
				Options:     c.Options,
				Selected:    c.Selected,
			})
		}
		scr := formScreen{id: "app/launch/form", form: form.NewWizard(groups)}
		return func() tea.Msg { return bus.ScreenPush{Model: scr} }
	case steps.GHAuth:
		scr := screens.NewGHAuth("app/launch/ghauth", p.Code, p.URL)
		authURL := p.URL
		ctx := a.ctx
		f := a.facade
		return tea.Batch(
			func() tea.Msg { return bus.ScreenPush{Model: scr} },
			func() tea.Msg { return openURLDoneMsg{err: f.OpenURL(ctx, authURL)} },
		)
	default:
		return nil
	}
}

func (a *App) handleTerminal(ev pipeline.Event) tea.Cmd {
	step := ev.Step
	argv := ev.Argv
	if step == "claude-auth" {
		tee, getToken := a.facade.NewTokenTee()
		cmd := exec.NewPTYTee(argv, tee)
		cmd.Env = ev.Env
		return execRunner(cmd, func(err error) tea.Msg {
			if err == nil {
				if token, ok := getToken(); ok {
					a.facade.OnTokenExtracted(token)
				}
			}
			return execDoneMsg{step: step, err: err}
		})
	}
	cmd := &exec.TTY{Argv: argv, Env: ev.Env}
	return execRunner(cmd, func(err error) tea.Msg {
		return execDoneMsg{step: step, err: err}
	})
}

func (a App) handleExecDone(msg execDoneMsg) (tea.Model, tea.Cmd) {
	if a.pipe == nil {
		return a, nil
	}
	_ = a.pipe.Resume(msg.step, pipeline.Result{})
	return a, nil
}

func (a App) handleScreenPop() (tea.Model, tea.Cmd) {
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPop{})
	if a.awaitingGate {
		a.awaitingGate = false
		return a.runGate(screens.GateSkip)
	}
	if a.awaitingRole {
		a.awaitingRole = false
		return a.backToMenu("")
	}
	if a.awaitingTune {
		a.awaitingTune = false
		if err := a.facade.ClearTune(); err != nil {
			return a.failToMenu(uistr.NoticeLoadoutErrPrefix + err.Error())
		}
		return a.maybeWarnRestart()
	}
	if a.awaitingRestart {
		a.awaitingRestart = false
		return a.backToMenu("")
	}
	step := a.waiting
	a.waiting = ""
	if a.pipe != nil && step != "" {
		a.launchCancelled = true
		_ = a.pipe.Resume(step, pipeline.Result{Cancelled: true})
	}
	return a, rc
}

func (a App) handlePipelineDone(msg pipelineDoneMsg) (tea.Model, tea.Cmd) {
	if a.pipe == nil {
		return a, nil
	}
	a.pipe = nil
	a.busy = false
	a.frame.SetBusy("")
	switch {
	case a.launchCancelled:
		a.launchCancelled = false
		return a.backToMenu(uistr.NoticeLaunchCanceled)
	case msg.failed:
		a.launchCancelled = false
		return a.failToMenu(uistr.NoticeLaunchFailed)
	}
	a.launchCancelled = false
	return a.backToMenu("")
}
