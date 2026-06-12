package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

const (
	ActionLaunch   = "launch"
	ActionHarness  = "harness"
	ActionTelegram = "telegram"
	ActionVSCode   = "vscode"
	ActionReset    = "reset"
	ActionQuit     = "quit"
)

func MenuItems() []frame.Item {
	return []frame.Item{
		{Title: uistr.MenuActionLaunch, Desc: uistr.MenuDescLaunch, Action: ActionLaunch, Enabled: true},
		{Title: uistr.MenuActionHarness, Desc: uistr.MenuDescHarness, Action: ActionHarness, Enabled: true},
		{Title: uistr.MenuActionTelegram, Desc: uistr.MenuDescTelegram, Action: ActionTelegram, Enabled: true},
		{Title: uistr.MenuActionVSCode, Desc: uistr.MenuDescVSCode, Action: ActionVSCode, Enabled: true},
		{Title: uistr.MenuActionReset, Desc: uistr.MenuDescReset, Action: ActionReset, Enabled: true},
		{Title: uistr.MenuActionQuit, Action: ActionQuit, Enabled: true},
	}
}

const menuDescColumn = 12

type Menu struct {
	id     bus.NodeID
	notice string
	items  []frame.Item
}

func NewMenu(id bus.NodeID) Menu {
	return Menu{id: id, items: MenuItems()}
}

func (m Menu) WithNotice(notice string) Menu {
	m.notice = notice
	return m
}

func (m Menu) Notice() string {
	return m.notice
}

func (m Menu) ID() bus.NodeID {
	return m.id
}

func (m Menu) Init() tea.Cmd {
	return nil
}

func (m Menu) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			return m, quitCmd
		}
	case bus.Envelope:
		return m.Update(msg.Msg)
	}
	return m, nil
}

func quitCmd() tea.Msg {
	return bus.MenuChosen{Action: ActionQuit}
}

func (m Menu) View() string {
	lines := []string{
		" " + styles.Title.Render(uistr.AppName),
		"",
	}
	for _, it := range m.items {
		if it.Desc == "" {
			continue
		}
		title := it.Title + strings.Repeat(" ", max(menuDescColumn-len(it.Title), 1))
		lines = append(lines, " "+styles.NormTitle.Render(title)+styles.Hint.Render(it.Desc))
	}
	lines = append(lines, "", " "+styles.Hint.Render(uistr.WelcomeHint))
	if m.notice != "" {
		lines = append(lines, "", " "+styles.Degraded.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}
