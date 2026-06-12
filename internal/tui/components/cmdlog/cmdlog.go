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

const maxLines = 5000

type Model struct {
	vp      viewport.Model
	ring    [maxLines]string
	head    int
	size    int
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

func (m *Model) add(line string) {
	if m.size < maxLines {
		m.ring[(m.head+m.size)%maxLines] = line
		m.size++
	} else {
		m.ring[m.head] = line
		m.head = (m.head + 1) % maxLines
	}
}

func (m Model) Lines() []string {
	out := make([]string, m.size)
	for i := range m.size {
		out[i] = m.ring[(m.head+i)%maxLines]
	}
	return out
}

func (m *Model) refresh() {
	lines := make([]string, m.size)
	for i := range m.size {
		lines[i] = m.ring[(m.head+i)%maxLines]
	}
	m.vp.SetContent(strings.Join(lines, "\n"))
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

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.StepEvent:
		added := false
		switch msg.Kind {
		case bus.StepStarted:
			if len(msg.Argv) > 0 {
				m.add(uistr.CmdlogPrefix + strings.Join(msg.Argv, " "))
				added = true
			}
		case bus.StepLine:
			if msg.Line != "" {
				m.add(msg.Line)
				added = true
			}
		}
		if added {
			m.refresh()
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
