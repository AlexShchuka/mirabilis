package app

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/ui"
)

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
	selTitle  = ui.SelTitleStyle
	normTitle = ui.NormTitleStyle
	hintStyle = ui.HintStyle
	offStyle  = ui.OffStyle
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

func newMenu(st provision.Status) menuModel {
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

func menuItems(st provision.Status) []list.Item {
	needUp := !st.ContainerUp
	gated := func(base string) (string, bool) {
		if needUp {
			return ui.MenuDescContainerOff, true
		}
		return base, false
	}
	hd, hDis := gated(ui.MenuDescHarness)
	vd, vDis := gated(ui.MenuDescVSCode)
	return []list.Item{
		item{action: "launch", title: ui.MenuActionLaunch, desc: ui.MenuDescLaunch},
		item{action: "harness", title: ui.MenuActionHarness, desc: hd, disabled: hDis},
		item{action: "vscode", title: ui.MenuActionVSCode, desc: vd, disabled: vDis},
		item{action: "reset", title: ui.MenuActionReset, desc: ui.MenuDescReset},
		item{action: "quit", title: ui.MenuActionQuit, desc: ""},
	}
}

func header(st provision.Status) string {
	parts := []string{"mirabilis"}
	if st.Stale {
		parts = append(parts, ui.MenuStale)
	}
	if st.CommitsBehind > 0 {
		parts = append(parts, fmt.Sprintf("%d behind origin/main", st.CommitsBehind))
	}
	switch st.Harness {
	case "off":
		parts = append(parts, ui.MenuHarnessOff)
	case "missing":
		parts = append(parts, ui.MenuHarnessMissing)
	case "unknown":
		parts = append(parts, ui.MenuHarnessUnknown)
	}
	if st.ProvisionWarn != "" {
		parts = append(parts, "provision: "+st.ProvisionWarn)
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

func (m menuModel) View() string {
	return m.list.View() + "\n " + hintStyle.Render(ui.MenuHint)
}
