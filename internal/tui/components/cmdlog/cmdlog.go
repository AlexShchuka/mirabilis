package cmdlog

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type Model struct {
	vp      viewport.Model
	lines   []string
	width   int
	focused bool
	follow  bool
}

func New() Model {
	return Model{vp: viewport.New(), follow: true}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.vp.SetWidth(w)
	m.vp.SetHeight(max(h-1, 0))
	m.refresh()
}

func (m *Model) Add(line string) {
	m.lines = append(m.lines, line)
	m.refresh()
}

func (m *Model) refresh() {
	m.vp.SetContent(strings.Join(m.lines, "\n"))
	if m.follow {
		m.vp.GotoBottom()
	}
}

func (m *Model) Focus() {
	m.focused = true
}

func (m *Model) Blur() {
	m.focused = false
}

func (m Model) Focused() bool {
	return m.focused
}

func (m Model) Following() bool {
	return m.follow
}

func (m Model) Lines() []string {
	return append([]string(nil), m.lines...)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.StepEvent:
		switch msg.Kind {
		case bus.StepStarted:
			if len(msg.Argv) > 0 {
				m.Add(uistr.CmdlogPrefix + strings.Join(msg.Argv, " "))
			}
		case bus.StepLine:
			if msg.Line != "" {
				m.Add(msg.Line)
			}
		}
		return m, nil
	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		m.follow = m.vp.AtBottom()
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	return m.titleRule() + "\n" + m.vp.View()
}

func (m Model) titleRule() string {
	rule := "─ " + uistr.CmdlogTitle + " "
	if pad := m.width - lipgloss.Width(rule); pad > 0 {
		rule += strings.Repeat("─", pad)
	}
	style := styles.CmdlogDim
	if m.focused {
		style = styles.SelTitle
	}
	return style.Render(rule)
}
