package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/steps"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func (a App) handleMenuChosen(msg bus.MenuChosen) (tea.Model, tea.Cmd) {
	if (a.busy || a.pipe != nil) && msg.Action != screens.ActionQuit {
		return a.backToMenu(uistr.NoticeBusy)
	}
	switch msg.Action {
	case screens.ActionLaunch:
		return a.pickRole()
	case screens.ActionQuit:
		a.cancel()
		return a, tea.Quit
	case screens.ActionVSCode:
		ctx := a.ctx
		f := a.facade
		a.busy = true
		tick := a.startBusy()
		m, _ := a.backToMenu(uistr.NoticeVSCodeOpening)
		return m, tea.Batch(tick, func() tea.Msg {
			return vscodeDoneMsg{err: f.OpenVSCode(ctx)}
		})
	case screens.ActionUpdate:
		ctx := a.ctx
		f := a.facade
		a.busy = true
		tick := a.startBusy()
		m, _ := a.backToMenu(uistr.NoticeUpdateRunning)
		return m, tea.Batch(tick, func() tea.Msg {
			return updateDoneMsg{err: f.UpdateEcosystem(ctx)}
		})
	default:
		return a, nil
	}
}

func (a App) handleScreenResult(msg bus.ScreenResult) (tea.Model, tea.Cmd) {
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPop{})
	if a.awaitingRole {
		a.awaitingRole = false
		name, ok := msg.Value.(string)
		if !ok {
			return a.backToMenu("")
		}
		if err := a.facade.SelectLoadout(name); err != nil {
			return a.failToMenu(uistr.NoticeLoadoutErrPrefix + err.Error())
		}
		return a.startLaunch()
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

func (a App) pickRole() (tea.Model, tea.Cmd) {
	if a.pipe != nil {
		return a, nil
	}
	choices := a.facade.Loadouts()
	if len(choices) == 0 {
		return a.startLaunch()
	}
	opts := make([]screens.RoleOption, 0, len(choices))
	for _, c := range choices {
		opts = append(opts, screens.RoleOption{
			Key:     c.Key,
			Effort:  c.Effort,
			Batch:   c.Batch,
			Default: c.Default,
		})
	}
	a.awaitingRole = true
	scr := screens.NewRolePicker("app/launch/role", opts)
	var rc tea.Cmd
	a.router, rc = a.router.Update(bus.ScreenPush{Model: scr})
	if a.winW > 0 || a.winH > 0 {
		mw, mh := a.frame.MainSize()
		a.router, _ = a.router.Update(tea.WindowSizeMsg{Width: mw, Height: mh})
	}
	return a, tea.Batch(rc, scr.Init())
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
	a.busy = true
	tick := a.startBusy()

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

	return a, tea.Batch(rc, ic, tick, pumpEvents(p.Events()))
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
