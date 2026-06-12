package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/steps"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/cmdlog"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/form"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/steplist"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

type statusMsg obs.Snapshot

type pipelineEventMsg struct {
	ev pipeline.Event
}

type pipelineDoneMsg struct {
	failed bool
}

type execDoneMsg struct {
	step string
	err  error
}

type resetDoneMsg struct {
	err error
}

func watchStatus(ch <-chan obs.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-ch
		if !ok {
			return nil
		}
		return statusMsg(snap)
	}
}

func pumpEvents(ch <-chan pipeline.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return pipelineDoneMsg{}
		}
		return pipelineEventMsg{ev: ev}
	}
}

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
		return a, tea.Batch(cmds...)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleWaiting(ev pipeline.Event) tea.Cmd {
	if len(ev.Argv) > 0 {
		return a.handleTerminal(ev)
	}
	switch p := ev.Payload.(type) {
	case steps.Catalog:
		scr := formScreen{
			id:   "app/launch/form",
			form: form.NewMultiSelect(p.Title, p.Options, p.Selected),
		}
		return func() tea.Msg { return bus.ScreenPush{Model: scr} }
	case steps.GHAuth:
		scr := screens.NewGHAuth("app/launch/ghauth", p.Code, p.URL)
		return func() tea.Msg { return bus.ScreenPush{Model: scr} }
	case steps.TelegramSetup:
		scr := screens.NewTelegram("app/launch/telegram")
		return func() tea.Msg { return bus.ScreenPush{Model: scr} }
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
		return execRunner(cmd, func(err error) tea.Msg {
			if err == nil {
				if token, ok := getToken(); ok {
					a.facade.OnTokenExtracted(token)
				}
			}
			return execDoneMsg{step: step, err: err}
		})
	}
	cmd := &exec.TTY{Argv: argv}
	return execRunner(cmd, func(err error) tea.Msg {
		return execDoneMsg{step: step, err: err}
	})
}

func (a App) handleExecDone(msg execDoneMsg) (tea.Model, tea.Cmd) {
	if a.pipe == nil {
		return a, nil
	}
	_ = a.pipe.Resume(msg.step, pipeline.Result{})
	if msg.step == "attach" {
		return a.backToMenu("")
	}
	return a, nil
}

func (a App) handleScreenResult(msg bus.ScreenResult) (tea.Model, tea.Cmd) {
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPop{})
	if a.menuAction != "" {
		action := a.menuAction
		a.menuAction = ""
		return a.handleMenuScreenResult(action, msg, rc)
	}
	step := a.waiting
	a.waiting = ""
	if a.pipe != nil && step != "" {
		_ = a.pipe.Resume(step, pipeline.Result{Value: msg.Value})
	}
	return a, rc
}

func (a App) handleMenuScreenResult(action string, msg bus.ScreenResult, popCmd tea.Cmd) (tea.Model, tea.Cmd) {
	switch action {
	case "reset":
		if v, ok := msg.Value.(bool); ok && v {
			ctx := a.ctx
			f := a.facade
			m, _ := a.backToMenu("")
			return m, tea.Cmd(func() tea.Msg {
				if err := f.SaveMemory(ctx); err != nil {
					return resetDoneMsg{err: err}
				}
				return resetDoneMsg{err: f.ResetSandbox(ctx)}
			})
		}
	}
	return a, popCmd
}

func (a App) handleResetDone(msg resetDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogResetFailed, "err", msg.err)
		return a.backToMenu(uistr.NoticeResetFailed)
	}
	return a.backToMenu(uistr.NoticeResetDone)
}

func (a App) handleScreenPop() (tea.Model, tea.Cmd) {
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPop{})
	if a.menuAction != "" {
		a.menuAction = ""
		return a.backToMenu("")
	}
	step := a.waiting
	a.waiting = ""
	if a.pipe != nil && step != "" {
		_ = a.pipe.Resume(step, pipeline.Result{Cancelled: true})
	}
	return a, rc
}

func (a App) handlePipelineDone(msg pipelineDoneMsg) (tea.Model, tea.Cmd) {
	a.pipe = nil
	notice := ""
	if msg.failed {
		notice = uistr.NoticeLaunchFailed
	}
	return a.backToMenu(notice)
}

func (a App) backToMenu(notice string) (tea.Model, tea.Cmd) {
	menu := screens.NewMenu("app/menu")
	if notice != "" {
		menu = menu.WithNotice(notice)
	}
	a.router = router.New(menu)
	return a, nil
}

func stepsToRows(cmds []pipeline.Command) []steplist.StepRow {
	rows := make([]steplist.StepRow, 0, len(cmds))
	for _, c := range cmds {
		m := c.Meta()
		rows = append(rows, steplist.StepRow{Name: m.Name, Title: m.Title})
	}
	return rows
}

func launchScreen(rows []steplist.StepRow) launchScr {
	sl := steplist.New(rows)
	cl := cmdlog.New()
	return launchScr{
		id:       "app/launch",
		steps:    sl,
		cmdlog:   cl,
		tabFocus: false,
	}
}

func (a App) handleMenuChosen(msg bus.MenuChosen) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case screens.ActionLaunch:
		return a.startLaunch()
	case screens.ActionQuit:
		a.cancel()
		return a, tea.Quit
	case screens.ActionReset:
		a.menuAction = "reset"
		scr := screens.NewReset("app/reset")
		var rc tea.Cmd
		a.router, rc = a.router.Update(bus.ScreenPush{Model: scr})
		return a, tea.Batch(rc, scr.Init())
	case screens.ActionVSCode:
		return a, nil
	case screens.ActionHarness:
		return a, nil
	default:
		return a, nil
	}
}

func (a App) startLaunch() (tea.Model, tea.Cmd) {
	if a.pipe != nil {
		return a, nil
	}
	pipeCmds := a.facade.LaunchSteps()
	p, err := pipeline.New(a.facade.Logger(), pipeCmds...)
	if err != nil {
		return a.backToMenu(uistr.NoticeLaunchErrPrefix + err.Error())
	}
	a.pipe = p

	rows := stepsToRows(pipeCmds)
	scr := launchScreen(rows)
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPush{Model: scr})

	if a.winW > 0 || a.winH > 0 {
		a.router, _ = a.router.Update(tea.WindowSizeMsg{Width: a.winW, Height: a.winH})
	}

	ic := scr.Init()
	pipeCtx := a.ctx
	go func() { _ = p.Run(pipeCtx) }()

	return a, tea.Batch(rc, ic, pumpEvents(p.Events()))
}

func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		if a.router.Depth() == 1 {
			a.cancel()
			return a, tea.Quit
		}
	case "esc":
		if a.router.Depth() > 1 {
			return a.handleScreenPop()
		}
	case "tab":
		a.router, _ = a.router.Update(toggleCmdlogMsg{})
		return a, nil
	}
	var cmd tea.Cmd
	a.router, cmd = a.router.Update(msg)
	return a, cmd
}
