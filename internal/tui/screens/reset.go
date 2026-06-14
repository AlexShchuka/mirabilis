package screens

import (
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/a11y"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type Reset struct {
	id   bus.NodeID
	form *huh.Form
	val  *bool
	done bool
}

func NewReset(id bus.NodeID) Reset {
	val := new(bool)
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(uistr.GlyphDanger + " " + uistr.FormTitleReset).
				Description(uistr.MenuDescReset).
				Affirmative(uistr.FormConfirmReset).
				Negative(uistr.FormCancelReset).
				Value(val),
		),
	).WithTheme(styles.HuhThemeDanger()).WithAccessible(a11y.Accessible())
	f.SubmitCmd = func() tea.Msg {
		if *val {
			return bus.ScreenResult{Value: true}
		}
		return bus.ScreenPop{}
	}
	f.CancelCmd = func() tea.Msg { return bus.ScreenPop{} }
	return Reset{id: id, form: f, val: val}
}

func (r Reset) ID() bus.NodeID { return r.id }

func (r Reset) Init() tea.Cmd { return r.form.Init() }

func (r Reset) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	if r.done {
		return r, nil
	}
	switch msg := msg.(type) {
	case bus.Envelope:
		return r.Update(msg.Msg)
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			r.done = true
			return r, func() tea.Msg { return bus.ScreenPop{} }
		}
	}
	m, cmd := r.form.Update(msg)
	if f, ok := m.(*huh.Form); ok {
		r.form = f
	}
	return r, cmd
}

func (r Reset) View() string {
	if r.done {
		return ""
	}
	return r.form.View()
}
