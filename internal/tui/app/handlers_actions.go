package app

import (
	"encoding/base64"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

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

func (a App) handleResetDone(msg resetDoneMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogResetFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeResetFailed)
	}
	return a.backToMenu(uistr.NoticeResetDone)
}

func (a App) handleCopyRequest(msg bus.CopyRequest) (tea.Model, tea.Cmd) {
	text := msg.Text
	ctx := a.ctx
	f := a.facade
	osc52 := osc52Copy(text)
	return a, tea.Batch(
		osc52,
		func() tea.Msg {
			return copyDoneMsg{text: text, err: f.CopyText(ctx, text)}
		},
	)
}

func (a App) handleCopyDone(msg copyDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return a, nil
	}
	_ = msg.text
	return a, nil
}

func osc52Copy(text string) tea.Cmd {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	return tea.Sequence(
		tea.Println("\x1b]52;c;" + encoded + "\x07"),
	)
}
