package app

import (
	"encoding/base64"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

const ghAuthNodeID = "app/launch/ghauth"

func (a App) handleVSCodeDone(msg vscodeDoneMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogVSCodeFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeVSCodeErr + msg.err.Error())
	}
	return a.backToMenu(uistr.NoticeVSCodeDone)
}

func (a App) handleUpdateDone(msg updateDoneMsg) (tea.Model, tea.Cmd) {
	a.busy = false
	if msg.err != nil {
		a.facade.Logger().Error(uistr.LogUpdateFailed, "err", msg.err)
		return a.failToMenu(uistr.NoticeUpdateErr + msg.err.Error())
	}
	return a.backToMenu(uistr.NoticeUpdateDone)
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
	line := uistr.GHAuthCopied
	if msg.err != nil {
		line = uistr.GHAuthCopyFailed
	}
	ev := bus.Envelope{
		To:  ghAuthNodeID,
		Msg: bus.StepEvent{Kind: bus.StepLine, Line: line},
	}
	var cmd tea.Cmd
	a.router, cmd = a.router.Update(ev)
	return a, cmd
}

func osc52Copy(text string) tea.Cmd {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	return tea.Sequence(
		tea.Println("\x1b]52;c;" + encoded + "\x07"),
	)
}
