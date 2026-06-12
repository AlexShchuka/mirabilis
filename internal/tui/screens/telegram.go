package screens

import (
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

const TelegramSkip = "skip"

type Telegram struct {
	id   bus.NodeID
	form *huh.Form
	tok  *string
	sel  *string
	done bool
}

func NewTelegram(id bus.NodeID, configured bool) Telegram {
	sel := new(string)
	tok := new(string)
	if configured {
		*sel = TelegramSkip
	}

	f := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(uistr.FormTitleTelegram).
				Options(
					huh.NewOption(uistr.FormOptTelegramConfigure, uistr.FormOptTelegramConfigure),
					huh.NewOption(uistr.FormOptTelegramSkip, TelegramSkip),
				).
				Value(sel),
		),
		huh.NewGroup(
			huh.NewInput().
				Title(uistr.FormTitleTelegramToken).
				Description(uistr.FormDescTelegramToken).
				EchoMode(huh.EchoModePassword).
				Value(tok),
		).WithHideFunc(func() bool { return *sel != uistr.FormOptTelegramConfigure }),
	)
	f.SubmitCmd = func() tea.Msg {
		if *sel == TelegramSkip || *tok == "" {
			return bus.ScreenResult{Value: TelegramSkip}
		}
		return bus.ScreenResult{Value: *tok}
	}
	f.CancelCmd = func() tea.Msg { return bus.ScreenPop{} }
	return Telegram{id: id, form: f, tok: tok, sel: sel}
}

func (t Telegram) ID() bus.NodeID { return t.id }

func (t Telegram) Init() tea.Cmd { return t.form.Init() }

func (t Telegram) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	if t.done {
		return t, nil
	}
	switch msg := msg.(type) {
	case bus.Envelope:
		return t.Update(msg.Msg)
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			t.done = true
			return t, func() tea.Msg { return bus.ScreenPop{} }
		}
	}
	m, cmd := t.form.Update(msg)
	if f, ok := m.(*huh.Form); ok {
		t.form = f
	}
	return t, cmd
}

func (t Telegram) View() string {
	if t.done {
		return ""
	}
	return t.form.View()
}
