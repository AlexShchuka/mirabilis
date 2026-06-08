package mainmenu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/list"

	"github.com/AlexShchuka/mirabilis/src/menu/internal/status"
)

type Model struct {
	list   list.Model
	action string
	st     status.Status
}

func New(st status.Status) Model {
	items := []list.Item{
		item{"launch", "Запустить", "запустить Claude в песочнице"},
		item{"update", "Обновить", "обновить mirabilis и пересобрать"},
		item{"plugins", "Плагины", "выбрать плагины Claude Code"},
		item{"harness", "Харнес", "neuro-matrix: вкл / выкл / переустановить"},
		item{"stacks", "Стек", "опциональные стеки сборки"},
		item{"vscode", "Открыть в VS Code", "подключить /workspace в VS Code"},
		item{"secrets", "Войти / секреты", "GitHub, Claude, context7, Telegram"},
		item{"theme", "Тема", "цветовая тема Claude"},
		item{"quit", "Выход", ""},
	}
	l := list.New(items, delegate{}, 0, 0)
	l.Title = header(st)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	return Model{list: l, st: st}
}

func header(st status.Status) string {
	parts := []string{"mirabilis"}
	if st.Stale {
		parts = append(parts, "workspace: stale (rebuild on launch)")
	}
	if st.CommitsBehind > 0 {
		parts = append(parts, fmt.Sprintf("mirabilis: %d behind origin/main", st.CommitsBehind))
	}
	switch st.Harness {
	case "off":
		parts = append(parts, "neuro-matrix: off")
	case "missing":
		parts = append(parts, "neuro-matrix: missing")
	}
	return strings.Join(parts, " · ")
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.action = "quit"
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(item); ok {
				m.action = it.action
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	return tea.NewView(m.list.View())
}

func Action(final tea.Model) string {
	if m, ok := final.(Model); ok {
		return m.action
	}
	return ""
}
