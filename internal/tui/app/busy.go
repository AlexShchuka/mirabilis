package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

const busyTickInterval = 120 * time.Millisecond

var busyFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type busyTickMsg struct {
	at  time.Time
	gen int
}

func busyTick(gen int) tea.Cmd {
	return tea.Tick(busyTickInterval, func(t time.Time) tea.Msg {
		return busyTickMsg{at: t, gen: gen}
	})
}

func (a App) busyText() string {
	glyph := busyFrames[a.busyFrame%len(busyFrames)]
	elapsed := int(time.Since(a.busyStarted).Seconds())
	return fmt.Sprintf("%s%s%ds", glyph, uistr.BusyElapsedSep, elapsed)
}

func (a *App) startBusy() tea.Cmd {
	a.busyGen++
	a.busyStarted = time.Now()
	a.busyFrame = 0
	a.frame.SetBusy(a.busyText())
	return busyTick(a.busyGen)
}

func (a App) handleBusyTick(msg busyTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != a.busyGen {
		return a, nil
	}
	if !a.busy {
		a.frame.SetBusy("")
		return a, nil
	}
	a.busyFrame++
	a.frame.SetBusy(a.busyText())
	return a, busyTick(a.busyGen)
}
