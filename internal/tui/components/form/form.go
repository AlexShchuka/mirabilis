package form

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/a11y"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type Group struct {
	Key         string
	Title       string
	Description string
	Options     []string
	Selected    []string
}

type Model struct {
	form   *huh.Form
	chosen map[string]*[]string
}

func NewWizard(groups []Group) Model {
	chosen := make(map[string]*[]string, len(groups))
	hg := make([]*huh.Group, 0, len(groups))
	for _, g := range groups {
		ptr := new([]string)
		chosen[g.Key] = ptr
		opts := make([]huh.Option[string], 0, len(g.Options))
		for _, o := range g.Options {
			opts = append(opts, huh.NewOption(o, o).Selected(slices.Contains(g.Selected, o)))
		}
		hg = append(hg, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(g.Title).
				Description(g.Description).
				Options(opts...).
				Filterable(false).
				Value(ptr),
		))
	}
	f := huh.NewForm(hg...).WithShowHelp(true).WithTheme(styles.HuhTheme()).WithAccessible(a11y.Accessible())
	f.SubmitCmd = func() tea.Msg {
		values := make(map[string][]string, len(chosen))
		for key, ptr := range chosen {
			values[key] = append([]string(nil), *ptr...)
		}
		return bus.ScreenResult{Values: values}
	}
	f.CancelCmd = popCmd
	return Model{form: f, chosen: chosen}
}

func popCmd() tea.Msg {
	return bus.ScreenPop{}
}

func (m Model) Init() tea.Cmd {
	return m.form.Init()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" {
		return m, popCmd
	}
	model, cmd := m.form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

func (m *Model) SetSize(w, h int) {
	m.form = m.form.WithWidth(w).WithHeight(h)
}

func (m Model) View() string {
	return m.form.View()
}
