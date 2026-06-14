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

func (a App) handlePromoted() (tea.Model, tea.Cmd) {
	if !a.secondary {
		return a, nil
	}
	a.promote()
	return a.backToMenu("")
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
