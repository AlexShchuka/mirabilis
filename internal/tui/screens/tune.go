package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type TuneResult struct {
	Effort string
	Fleet  bool
}

var effortCycle = []string{
	uistr.EffortLow,
	uistr.EffortMedium,
	uistr.EffortHigh,
	uistr.EffortXHigh,
	uistr.EffortMax,
}

const (
	tuneRowEffort = 0
	tuneRowFleet  = 1
	tuneRows      = 2
)

type Tune struct {
	id     bus.NodeID
	effort string
	row    int
	fleet  bool
}

var _ router.Screen = Tune{}

func NewTune(id bus.NodeID, effort string, fleet bool) Tune {
	return Tune{id: id, effort: normalizeEffort(effort), fleet: fleet}
}

func normalizeEffort(effort string) string {
	for _, e := range effortCycle {
		if e == effort {
			return effort
		}
	}
	return uistr.EffortMedium
}

func effortIndex(effort string) int {
	for i, e := range effortCycle {
		if e == effort {
			return i
		}
	}
	return 0
}

func (t Tune) ID() bus.NodeID { return t.id }

func (t Tune) Init() tea.Cmd { return nil }

func (t Tune) moveRow(delta int) Tune {
	target := t.row + delta
	if target >= 0 && target < tuneRows {
		t.row = target
	}
	return t
}

func (t Tune) cycleEffort(delta int) Tune {
	i := effortIndex(t.effort) + delta
	if i < 0 {
		i = 0
	}
	if i >= len(effortCycle) {
		i = len(effortCycle) - 1
	}
	t.effort = effortCycle[i]
	return t
}

func (t Tune) adjust(delta int) Tune {
	switch t.row {
	case tuneRowEffort:
		return t.cycleEffort(delta)
	case tuneRowFleet:
		t.fleet = !t.fleet
	}
	return t
}

func (t Tune) result() tea.Cmd {
	res := TuneResult{Effort: t.effort, Fleet: t.fleet}
	return func() tea.Msg { return bus.ScreenResult{Value: res} }
}

func (t Tune) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.Envelope:
		return t.Update(msg.Msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			return t.moveRow(-1), nil
		case "down", "j":
			return t.moveRow(1), nil
		case "left", "h":
			return t.adjust(-1), nil
		case "right", "l", "space":
			return t.adjust(1), nil
		case "enter":
			return t, t.result()
		case "esc":
			return t, func() tea.Msg { return bus.ScreenPop{} }
		}
	}
	return t, nil
}

func (t Tune) fleetValue() string {
	if t.fleet {
		return uistr.TuneFleetOn
	}
	return uistr.TuneFleetOff
}

func (t Tune) row1(active bool, label, value string) string {
	style := styles.NormTitle
	if active {
		style = styles.SelTitle
	}
	return "  " + styles.Hint.Render(label) + " " + style.Render(value)
}

func (t Tune) View() string {
	var b strings.Builder
	b.WriteString(" " + styles.Title.Render(uistr.TuneTitle) + "\n")
	b.WriteString(" " + styles.Hint.Render(uistr.TuneLead) + "\n\n")
	b.WriteString(t.row1(t.row == tuneRowEffort, uistr.TuneEffortLabel, t.effort) + "\n")
	b.WriteString(t.row1(t.row == tuneRowFleet, uistr.TuneFleetLabel, t.fleetValue()) + "\n")
	b.WriteString("\n " + styles.Hint.Render(uistr.TuneHint))
	return b.String()
}
