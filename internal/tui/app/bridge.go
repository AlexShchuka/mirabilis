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

type telegramDoneMsg struct {
	err error
}

type harnessStatusMsg struct {
	current string
	err     error
}

type harnessDoneMsg struct {
	err error
}

type vscodeDoneMsg struct {
	err error
}

type attachReadyMsg struct {
	argv []string
	env  []string
	err  error
}

type promotedMsg struct{}

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
		return func() tea.Msg { return bus.ScreenPush{Model: scr} }
	case steps.TelegramSetup:
		scr := screens.NewTelegram("app/launch/telegram", a.facade.TelegramConfigured())
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
		_ = a.pipe.Resume(step, pipeline.Result{Value: screenResultValue(msg)})
	}
	return a, rc
}

func screenResultValue(msg bus.ScreenResult) any {
	if msg.Values != nil {
		return steps.WizardResult{Choices: msg.Values}
	}
	return msg.Value
}

func (a App) handleMenuScreenResult(action string, msg bus.ScreenResult, popCmd tea.Cmd) (tea.Model, tea.Cmd) {
	switch action {
	case "reset":
		if v, ok := msg.Value.(bool); ok && v {
			ctx := a.ctx
			f := a.facade
			a.busy = true
			tick := a.startBusy()
			m, _ := a.backToMenu("")
			return m, tea.Batch(tick, func() tea.Msg {
				if err := f.SaveMemory(ctx); err != nil {
					return resetDoneMsg{err: err}
				}
				return resetDoneMsg{err: f.ResetSandbox(ctx)}
			})
		}
	case "telegram":
		if token, ok := msg.Value.(string); ok && token != screens.TelegramSkip {
			ctx := a.ctx
			f := a.facade
			a.busy = true
			tick := a.startBusy()
			m, _ := a.backToMenu(uistr.NoticeTelegramConfiguring)
			return m, tea.Batch(tick, func() tea.Msg {
				return telegramDoneMsg{err: f.ConfigureTelegram(ctx, token)}
			})
		}
	case "harness":
		if choice, ok := msg.Value.(string); ok && choice != "" {
			ctx := a.ctx
			f := a.facade
			a.busy = true
			a.harnessChoice = choice
			tick := a.startBusy()
			m, _ := a.backToMenu(uistr.NoticeHarnessApplying)
			return m, tea.Batch(tick, func() tea.Msg {
				return harnessDoneMsg{err: f.ApplyHarness(ctx, choice)}
			})
		}
	}
	return a, popCmd
}

func (a App) handleTelegramDone(msg telegramDoneMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogTelegramFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeTelegramErr + msg.err.Error())
	}
	m, _ := a.backToMenu(uistr.NoticeTelegramDone)
	return m, a.rememberTelegram()
}

func (a App) handleHarnessStatus(msg harnessStatusMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	a.frame.SetBusy("")
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogHarnessFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeHarnessErr + msg.err.Error())
	}
	a.menuAction = "harness"
	scr := screens.NewHarness("app/harness", msg.current, a.facade.LastHarnessChoice())
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPush{Model: scr})
	return a, tea.Batch(rc, scr.Init())
}

func (a App) handleHarnessDone(msg harnessDoneMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogHarnessFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeHarnessErr + msg.err.Error())
	}
	choice := a.harnessChoice
	m, _ := a.backToMenu(uistr.NoticeHarnessDone)
	return m, a.rememberHarness(choice)
}

func (a App) handleVSCodeDone(msg vscodeDoneMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogVSCodeFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeVSCodeErr + msg.err.Error())
	}
	return a.backToMenu(uistr.NoticeVSCodeDone)
}

func (a App) startAttach() (tea.Model, tea.Cmd) {
	ctx := a.ctx
	f := a.facade
	a.busy = true
	tick := a.startBusy()
	m, _ := a.backToMenu(uistr.NoticeAttachOpening)
	return m, tea.Batch(tick, func() tea.Msg {
		argv, env, err := f.AttachExec(ctx)
		return attachReadyMsg{argv: argv, env: env, err: err}
	})
}

func (a App) handleAttachReady(msg attachReadyMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogAttachFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeAttachErr + msg.err.Error())
	}
	m, _ := a.backToMenu("")
	cmd := &exec.TTY{Argv: msg.argv, Env: msg.env}
	return m, execRunner(cmd, func(err error) tea.Msg {
		return execDoneMsg{step: "", err: err}
	})
}

func (a App) handlePromoted() (tea.Model, tea.Cmd) {
	if !a.secondary {
		return a, nil
	}
	a.promote()
	return a.backToMenu("")
}

func (a App) handleResetDone(msg resetDoneMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogResetFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeResetFailed)
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

func (a App) rememberHarness(choice string) tea.Cmd {
	f := a.facade
	log := a.facade.Logger()
	return func() tea.Msg {
		if err := f.RememberHarnessChoice(choice); err != nil {
			log.Error(uistr.LogHarnessFailed, "err", err)
		}
		return nil
	}
}

func (a App) rememberTelegram() tea.Cmd {
	f := a.facade
	log := a.facade.Logger()
	return func() tea.Msg {
		if err := f.MarkTelegramConfigured(); err != nil {
			log.Error(uistr.LogTelegramFailed, "err", err)
		}
		return nil
	}
}

func (a App) backToMenu(notice string) (tea.Model, tea.Cmd) {
	if !a.busy {
		a.frame.SetBusy("")
	}
	if notice == "" {
		notice = a.baseNotice
	}
	menu := screens.NewMenu("app/menu")
	if notice != "" {
		menu = menu.WithNotice(notice)
	}
	if a.errNotice != "" {
		menu = menu.WithError(a.errNotice)
	}
	a.router = router.New(menu)
	return a, nil
}

func (a App) failToMenu(notice string) (tea.Model, tea.Cmd) {
	a.errNotice = notice
	return a.backToMenu(notice)
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
	if (a.busy || a.pipe != nil) && msg.Action != screens.ActionQuit {
		return a.backToMenu(uistr.NoticeBusy)
	}
	switch msg.Action {
	case screens.ActionLaunch:
		return a.startLaunch()
	case screens.ActionAttach:
		return a.startAttach()
	case screens.ActionQuit:
		a.cancel()
		return a, tea.Quit
	case screens.ActionReset:
		a.menuAction = "reset"
		scr := screens.NewReset("app/reset")
		var rc tea.Cmd
		a.router, rc = a.router.Update(bus.ScreenPush{Model: scr})
		return a, tea.Batch(rc, scr.Init())
	case screens.ActionTelegram:
		a.menuAction = "telegram"
		scr := screens.NewTelegram("app/telegram", a.facade.TelegramConfigured())
		var rc tea.Cmd
		a.router, rc = a.router.Update(bus.ScreenPush{Model: scr})
		return a, tea.Batch(rc, scr.Init())
	case screens.ActionHarness:
		ctx := a.ctx
		f := a.facade
		a.busy = true
		tick := a.startBusy()
		return a, tea.Batch(tick, func() tea.Msg {
			current, err := f.HarnessStatus(ctx)
			return harnessStatusMsg{current: current, err: err}
		})
	case screens.ActionVSCode:
		ctx := a.ctx
		f := a.facade
		a.busy = true
		tick := a.startBusy()
		m, _ := a.backToMenu(uistr.NoticeVSCodeOpening)
		return m, tea.Batch(tick, func() tea.Msg {
			return vscodeDoneMsg{err: f.OpenVSCode(ctx)}
		})
	default:
		return a, nil
	}
}

func (a App) startLaunch() (tea.Model, tea.Cmd) {
	if a.pipe != nil {
		return a, nil
	}
	a.launchCancelled = false
	pipeCmds := a.facade.LaunchSteps()
	p, err := pipeline.New(a.facade.Logger(), pipeCmds...)
	if err != nil {
		return a.failToMenu(uistr.NoticeLaunchErrPrefix + err.Error())
	}
	a.pipe = p

	rows := stepsToRows(pipeCmds)
	scr := launchScreen(rows)
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPush{Model: scr})

	if a.winW > 0 || a.winH > 0 {
		mw, mh := a.frame.MainSize()
		a.router, _ = a.router.Update(tea.WindowSizeMsg{Width: mw, Height: mh})
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
	case "x":
		if a.router.Depth() == 1 && a.errNotice != "" {
			a.errNotice = ""
			return a.backToMenu("")
		}
	case "esc":
		if a.router.Depth() > 1 {
			return a.handleScreenPop()
		}
	case "tab":
		a.router, _ = a.router.Update(toggleCmdlogMsg{})
		return a, nil
	case "up", "k", "down", "j", "enter":
		if a.router.Depth() == 1 {
			var cmd tea.Cmd
			a.frame, cmd = a.frame.Update(msg)
			return a, cmd
		}
	}
	var cmd tea.Cmd
	a.router, cmd = a.router.Update(msg)
	return a, cmd
}
