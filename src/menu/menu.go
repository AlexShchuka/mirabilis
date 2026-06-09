package main

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Status struct {
	CommitsBehind int
	Stale         bool
	Harness       string
	ContainerUp   bool
}

type item struct {
	action   string
	title    string
	desc     string
	disabled bool
}

func (i item) FilterValue() string { return i.title }
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }

const titleWidth = 19

var (
	selTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	normTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	offStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type delegate struct{}

func (d delegate) Height() int                             { return 1 }
func (d delegate) Spacing() int                            { return 0 }
func (d delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d delegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	row, ok := li.(item)
	if !ok {
		return
	}
	cursor, ts, ds := "  ", normTitle, hintStyle
	switch {
	case row.disabled:
		ts, ds = offStyle, offStyle
	case index == m.Index():
		cursor, ts = "> ", selTitle
	}
	out := cursor + ts.Width(titleWidth).Render(row.title)
	if row.desc != "" {
		out += ds.Render(row.desc)
	}
	fmt.Fprint(w, out)
}

type menuModel struct {
	list list.Model
}

func newMenu(st Status) menuModel {
	l := list.New(menuItems(st), delegate{}, 0, 0)
	l.Title = header(st)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	m := menuModel{list: l}
	m.skipDisabled(+1)
	return m
}

func menuItems(st Status) []list.Item {
	needUp := !st.ContainerUp
	gated := func(base string) (string, bool) {
		if needUp {
			return "контейнер не запущен — сначала «Запустить»", true
		}
		return base, false
	}
	pd, pDis := gated("выбрать плагины Claude Code")
	hd, hDis := gated("neuro-matrix: вкл / выкл / переустановить")
	vd, vDis := gated("подключить /workspace в VS Code")
	return []list.Item{
		item{action: "launch", title: "Запустить", desc: "пайплайн настройки + Claude в контейнере"},
		item{action: "plugins", title: "Плагины", desc: pd, disabled: pDis},
		item{action: "harness", title: "Харнес", desc: hd, disabled: hDis},
		item{action: "stacks", title: "Стек", desc: "опциональные стеки сборки"},
		item{action: "vscode", title: "Открыть в VS Code", desc: vd, disabled: vDis},
		item{action: "quit", title: "Выход", desc: ""},
	}
}

func header(st Status) string {
	parts := []string{"mirabilis"}
	if st.Stale {
		parts = append(parts, "workspace: stale (rebuild on launch)")
	}
	if st.CommitsBehind > 0 {
		parts = append(parts, fmt.Sprintf("%d behind origin/main", st.CommitsBehind))
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

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (menuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s := m.list.Styles
		titleH := lipgloss.Height(s.TitleBar.Render(s.Title.Render(m.list.Title)))
		var d delegate
		rows := len(m.list.Items()) * (d.Height() + d.Spacing())
		m.list.SetSize(msg.Width, min(msg.Height, titleH+rows+1))
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			return m, emit(menuChoiceMsg{"quit"})
		case "enter":
			if it, ok := m.list.SelectedItem().(item); ok && !it.disabled {
				return m, emit(menuChoiceMsg{it.action})
			}
			return m, nil
		case "up", "k":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			m.skipDisabled(-1)
			return m, cmd
		case "down", "j":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			m.skipDisabled(+1)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *menuModel) skipDisabled(dir int) {
	n := len(m.list.Items())
	for tries := 0; tries < n; tries++ {
		it, ok := m.list.SelectedItem().(item)
		if !ok || !it.disabled {
			return
		}
		if dir < 0 {
			m.list.CursorUp()
		} else {
			m.list.CursorDown()
		}
	}
}

func (m menuModel) selected() (item, bool) {
	it, ok := m.list.SelectedItem().(item)
	return it, ok
}

func (m menuModel) View() string {
	return m.list.View() + "\n " + hintStyle.Render("enter · q выход")
}
