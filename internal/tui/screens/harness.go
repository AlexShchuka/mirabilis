package screens

import (
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

const (
	HarnessOn        = "on"
	HarnessOff       = "off"
	HarnessReinstall = "reinstall"
)

type Harness struct {
	id      bus.NodeID
	form    *huh.Form
	val     *string
	current string
	done    bool
}

func NewHarness(id bus.NodeID, current string) Harness {
	val := new(string)
	currentLabel := harnessLabel(current)
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(uistr.FormTitleHarness).
				Description(currentLabel).
				Options(
					huh.NewOption(uistr.FormOptHarnessOn, HarnessOn),
					huh.NewOption(uistr.FormOptHarnessOff, HarnessOff),
					huh.NewOption(uistr.FormOptHarnessRe, HarnessReinstall),
				).
				Value(val),
		),
	)
	f.SubmitCmd = func() tea.Msg {
		return bus.ScreenResult{Value: *val}
	}
	f.CancelCmd = func() tea.Msg { return bus.ScreenPop{} }
	return Harness{id: id, form: f, val: val, current: current}
}

func harnessLabel(current string) string {
	switch current {
	case HarnessOff:
		return uistr.MenuHarnessOff
	case "missing":
		return uistr.MenuHarnessMissing
	default:
		return uistr.MenuHarnessUnknown
	}
}

func (h Harness) ID() bus.NodeID { return h.id }

func (h Harness) Init() tea.Cmd { return h.form.Init() }

func (h Harness) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	if h.done {
		return h, nil
	}
	switch msg := msg.(type) {
	case bus.Envelope:
		return h.Update(msg.Msg)
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			h.done = true
			return h, func() tea.Msg { return bus.ScreenPop{} }
		}
	}
	m, cmd := h.form.Update(msg)
	if f, ok := m.(*huh.Form); ok {
		h.form = f
	}
	return h, cmd
}

func (h Harness) View() string {
	if h.done {
		return ""
	}
	return h.form.View()
}
