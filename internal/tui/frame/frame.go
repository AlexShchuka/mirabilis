package frame

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/statusbar"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

const MenuWidth = 18

type Item struct {
	Title   string
	Desc    string
	Action  string
	Enabled bool
}

type Model struct {
	title   string
	version string
	items   []Item
	status  statusbar.Model
	cursor  int
	width   int
	height  int
}

func New(title, version string, items []Item) Model {
	m := Model{
		title:   title,
		version: version,
		items:   append([]Item(nil), items...),
		status:  statusbar.New(),
		width:   80,
		height:  24,
	}
	m.cursor = m.firstEnabled()
	return m
}

func (m Model) firstEnabled() int {
	for i, it := range m.items {
		if it.Enabled {
			return i
		}
	}
	return 0
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case bus.StatusChanged:
		m.status = m.status.Update(msg)
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(+1)
		case "enter":
			if it, ok := m.Selected(); ok && it.Enabled {
				return m, chooseCmd(it.Action)
			}
		}
		return m, nil
	}
	return m, nil
}

func chooseCmd(action string) tea.Cmd {
	return func() tea.Msg {
		return bus.MenuChosen{Action: action}
	}
}

func (m *Model) move(dir int) {
	for i := m.cursor + dir; i >= 0 && i < len(m.items); i += dir {
		if m.items[i].Enabled {
			m.cursor = i
			return
		}
	}
}

func (m Model) Selected() (Item, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return Item{}, false
	}
	return m.items[m.cursor], true
}

func (m *Model) SetEnabled(action string, enabled bool) {
	items := append([]Item(nil), m.items...)
	m.items = items
	for i := range items {
		if items[i].Action == action {
			items[i].Enabled = enabled
		}
	}
	if it, ok := m.Selected(); ok && !it.Enabled {
		m.cursor = m.firstEnabled()
	}
}

func (m Model) MainSize() (int, int) {
	return max(m.width-MenuWidth-1, 0), max(m.height-2, 0)
}

func (m Model) View(main string) string {
	mw, mh := m.MainSize()
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.menuView(mh),
		m.separator(mh),
		lipgloss.NewStyle().Width(mw).Height(mh).MaxHeight(mh).Render(main),
	)
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body, m.footerView())
}

func (m Model) headerView() string {
	left := " " + styles.Header.Render(m.title)
	right := ""
	if m.version != "" {
		right = styles.HeaderRight.Render(m.version)
	}
	if sv := m.status.View(); sv != "" {
		if right != "" {
			right += uistr.StatusSep
		}
		right += sv
	}
	if right != "" {
		right += " "
	}
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return lipgloss.NewStyle().MaxWidth(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m Model) menuView(h int) string {
	lines := make([]string, 0, len(m.items))
	for i, it := range m.items {
		cursor, style := "  ", styles.MenuNormal
		switch {
		case !it.Enabled:
			style = styles.MenuDisabled
		case i == m.cursor:
			cursor, style = "> ", styles.MenuSelected
		}
		lines = append(lines, cursor+style.Render(it.Title))
	}
	return lipgloss.NewStyle().Width(MenuWidth).Height(h).MaxHeight(h).Render(strings.Join(lines, "\n"))
}

func (m Model) separator(h int) string {
	if h <= 0 {
		return ""
	}
	col := strings.TrimSuffix(strings.Repeat("│\n", h), "\n")
	return styles.MainBorder.Render(col)
}

func (m Model) footerView() string {
	return lipgloss.NewStyle().MaxWidth(m.width).Render(" " + styles.Hint.Render(uistr.FooterHints))
}
