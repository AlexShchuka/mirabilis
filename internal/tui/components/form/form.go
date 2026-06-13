package form

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
)

type Model struct {
	form   *huh.Form
	chosen *[]string
}

func NewMultiSelect(title, description string, options, selected []string) Model {
	chosen := new([]string)
	opts := make([]huh.Option[string], 0, len(options))
	for _, o := range options {
		opts = append(opts, huh.NewOption(o, o).Selected(slices.Contains(selected, o)))
	}
	f := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title(title).
			Description(description).
			Options(opts...).
			Filterable(false).
			Value(chosen),
	))
	f.SubmitCmd = func() tea.Msg {
		return bus.ScreenResult{Value: append([]string(nil), *chosen...)}
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
