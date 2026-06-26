package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

const (
	RestartConfirm = "restart-confirm"
	RestartCancel  = "restart-cancel"
)

type warnOption struct {
	key   string
	label string
	desc  string
}

func warnOptions() []warnOption {
	return []warnOption{
		{key: RestartCancel, label: uistr.RestartWarnCancel, desc: uistr.RestartWarnCancelD},
		{key: RestartConfirm, label: uistr.RestartWarnConfirm, desc: uistr.RestartWarnConfirmD},
	}
}

type RestartWarn struct {
	id      bus.NodeID
	options []warnOption
	cursor  int
}

var _ router.Screen = RestartWarn{}

func NewRestartWarn(id bus.NodeID) RestartWarn {
	return RestartWarn{id: id, options: warnOptions()}
}

func (w RestartWarn) ID() bus.NodeID { return w.id }

func (w RestartWarn) Init() tea.Cmd { return nil }

func (w RestartWarn) move(delta int) RestartWarn {
	target := w.cursor + delta
	if target >= 0 && target < len(w.options) {
		w.cursor = target
	}
	return w
}

func (w RestartWarn) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.Envelope:
		return w.Update(msg.Msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			return w.move(-1), nil
		case "down", "j":
			return w.move(1), nil
		case "enter":
			key := w.options[w.cursor].key
			return w, func() tea.Msg { return bus.ScreenResult{Value: key} }
		case "esc":
			return w, func() tea.Msg { return bus.ScreenResult{Value: RestartCancel} }
		}
	}
	return w, nil
}

func (w RestartWarn) View() string {
	var b strings.Builder
	b.WriteString(" " + styles.Danger.Render(uistr.GlyphDanger+" "+uistr.RestartWarnTitle) + "\n")
	b.WriteString(" " + styles.Hint.Render(uistr.RestartWarnLead) + "\n")
	b.WriteString(" " + styles.Degraded.Render(uistr.RestartWarnLost) + "\n")
	b.WriteString(" " + styles.OK.Render(uistr.RestartWarnKept) + "\n\n")
	for i, o := range w.options {
		style := styles.NormTitle
		if i == w.cursor {
			style = styles.SelTitle
		}
		b.WriteString("  " + style.Render(o.label) + styles.Hint.Render(uistr.StatusSep+o.desc) + "\n")
	}
	b.WriteString("\n " + styles.Hint.Render(uistr.RestartWarnHint))
	return b.String()
}
