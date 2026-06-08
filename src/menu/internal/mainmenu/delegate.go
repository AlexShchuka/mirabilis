package mainmenu

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type item struct {
	action string
	title  string
	desc   string
}

func (i item) FilterValue() string { return i.title }
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }

type delegate struct{}

func (d delegate) Height() int                             { return 2 }
func (d delegate) Spacing() int                            { return 1 }
func (d delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

var (
	selTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	selDesc   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	normTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	normDesc  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (d delegate) Render(w io.Writer, m list.Model, index int, it list.Item) {
	row, ok := it.(item)
	if !ok {
		return
	}
	cursor := "  "
	title, desc := normTitle, normDesc
	if index == m.Index() {
		cursor = "> "
		title, desc = selTitle, selDesc
	}
	var b strings.Builder
	b.WriteString(cursor + title.Render(row.title) + "\n")
	b.WriteString("  " + desc.Render(row.desc))
	fmt.Fprint(w, b.String())
}
