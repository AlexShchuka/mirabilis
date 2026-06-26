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
	GateSkip  = "skip"
	GateSelf  = "self"
	GatePacks = "packs"
	GateAll   = "all"
)

type gateOption struct {
	key   string
	label string
	desc  string
}

func gateOptions() []gateOption {
	return []gateOption{
		{key: GateSkip, label: uistr.GateOptionSkip, desc: uistr.GateDescSkip},
		{key: GateSelf, label: uistr.GateOptionSelf, desc: uistr.GateDescSelf},
		{key: GatePacks, label: uistr.GateOptionPacks, desc: uistr.GateDescPacks},
		{key: GateAll, label: uistr.GateOptionAll, desc: uistr.GateDescAll},
	}
}

type UpdateGate struct {
	id       bus.NodeID
	current  string
	outdated string
	options  []gateOption
	cursor   int
}

var _ router.Screen = UpdateGate{}

func NewUpdateGate(id bus.NodeID, current, outdated string) UpdateGate {
	return UpdateGate{id: id, current: current, outdated: outdated, options: gateOptions()}
}

func (g UpdateGate) ID() bus.NodeID { return g.id }

func (g UpdateGate) Init() tea.Cmd { return nil }

func (g UpdateGate) move(delta int) UpdateGate {
	target := g.cursor + delta
	if target >= 0 && target < len(g.options) {
		g.cursor = target
	}
	return g
}

func (g UpdateGate) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.Envelope:
		return g.Update(msg.Msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			return g.move(-1), nil
		case "down", "j":
			return g.move(1), nil
		case "enter":
			key := g.options[g.cursor].key
			return g, func() tea.Msg { return bus.ScreenResult{Value: key} }
		case "esc":
			return g, func() tea.Msg { return bus.ScreenResult{Value: GateSkip} }
		}
	}
	return g, nil
}

func (g UpdateGate) freshness() string {
	parts := make([]string, 0, 2)
	if g.current != "" {
		parts = append(parts, styles.HeaderRight.Render(uistr.GateCurrentPrefix+g.current))
	}
	if g.outdated != "" {
		parts = append(parts, styles.Degraded.Render(uistr.GateOutdatedPrefix+g.outdated+uistr.GateOutdatedSuffix))
	} else {
		parts = append(parts, styles.OK.Render(uistr.GateUpToDate))
	}
	return strings.Join(parts, uistr.StatusSep)
}

func (g UpdateGate) View() string {
	var b strings.Builder
	b.WriteString(" " + styles.Title.Render(uistr.GateTitle) + "\n")
	b.WriteString(" " + g.freshness() + "\n\n")
	for i, o := range g.options {
		cursor, style := "  ", styles.NormTitle
		if i == g.cursor {
			cursor, style = "▸ ", styles.SelTitle
		}
		b.WriteString(cursor + style.Render(o.label) + styles.Hint.Render(uistr.StatusSep+o.desc) + "\n")
	}
	b.WriteString("\n " + styles.Hint.Render(uistr.GateHint))
	return b.String()
}
