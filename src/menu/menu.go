package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Status is the menu's view of the workspace, fed as JSON on stdin by the bash orchestrator.
type Status struct {
	CommitsBehind int    `json:"commitsBehind"`
	Stale         bool   `json:"stale"`
	Harness       string `json:"harness"`
}

func FromStdin() Status {
	var s Status
	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) != 0 {
		return s
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

type item struct {
	action string
	title  string
	desc   string
}

func (i item) FilterValue() string { return i.title }
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }

const titleWidth = 19

var (
	selTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	normTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

type delegate struct{}

func (d delegate) Height() int                             { return 1 }
func (d delegate) Spacing() int                            { return 0 }
func (d delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d delegate) Render(w io.Writer, m list.Model, index int, it list.Item) {
	row, ok := it.(item)
	if !ok {
		return
	}
	cursor := "  "
	ts := normTitle
	if index == m.Index() {
		cursor = "> "
		ts = selTitle
	}
	out := cursor + ts.Width(titleWidth).Render(row.title)
	if row.desc != "" {
		out += hintStyle.Render(row.desc)
	}
	fmt.Fprint(w, out)
}

type Model struct {
	list   list.Model
	action string
}

func New(st Status) Model {
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
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	return Model{list: l}
}

func header(st Status) string {
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
	case "unknown":
		parts = append(parts, "neuro-matrix: unknown")
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
	return tea.NewView(m.list.View() + "\n " + hintStyle.Render("enter · q выход"))
}

func Action(final tea.Model) string {
	if m, ok := final.(Model); ok {
		return m.action
	}
	return ""
}
