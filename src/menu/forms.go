package main

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
)

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func joinCSV(items []string) string { return strings.Join(items, ",") }

type formScreen struct {
	form  *huh.Form
	apply func() tea.Cmd
}

func (f *formScreen) Init() tea.Cmd { return f.form.Init() }

func (f *formScreen) Update(msg tea.Msg) (*formScreen, tea.Cmd) {
	model, cmd := f.form.Update(msg)
	if hf, ok := model.(*huh.Form); ok {
		f.form = hf
	}
	return f, cmd
}

func (f *formScreen) View() string    { return f.form.View() }
func (f *formScreen) completed() bool { return f.form.State == huh.StateCompleted }
func (f *formScreen) aborted() bool   { return f.form.State == huh.StateAborted }

func newPluginsForm(ctx context.Context, r Runner, w, h int) *formScreen {
	catalog := pluginCatalog(ctx, r)
	if len(catalog) == 0 {
		return nil
	}
	disabled := pluginsDisabled(ctx, r)
	opts := make([]huh.Option[string], 0, len(catalog))
	for _, p := range catalog {
		opts = append(opts, huh.NewOption(p, p).Selected(!contains(disabled, p)))
	}
	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Плагины (пробел — переключить, Enter — ок)").
			Options(opts...).Value(&chosen),
	)).WithWidth(w).WithHeight(h)
	apply := func() tea.Cmd {
		return func() tea.Msg {
			var newDisabled []string
			for _, p := range catalog {
				if !contains(chosen, p) {
					newDisabled = append(newDisabled, p)
				}
			}
			if err := writePluginsDisabled(ctx, r, newDisabled); err != nil {
				return backToMenuMsg{notice: "плагины: " + err.Error()}
			}
			return backToMenuMsg{}
		}
	}
	return &formScreen{form: form, apply: apply}
}

func newHarnessForm(ctx context.Context, r Runner, w, h int) *formScreen {
	current := "on"
	if pref, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`); strings.TrimSpace(pref) == "skip" {
		current = "off"
	}
	choice := current
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("neuro-matrix харнес").
			Options(
				huh.NewOption("Включить", "on"),
				huh.NewOption("Выключить", "off"),
				huh.NewOption("Переустановить", "reinstall"),
			).Value(&choice),
	)).WithWidth(w).WithHeight(h)
	apply := func() tea.Cmd {
		return func() tea.Msg {
			if err := applyHarness(ctx, r, choice); err != nil {
				return backToMenuMsg{notice: "харнес: " + err.Error()}
			}
			return backToMenuMsg{}
		}
	}
	return &formScreen{form: form, apply: apply}
}

func newStacksForm(r Runner, w, h int) *formScreen {
	catalog := readStackCatalog(r.Repo())
	if len(catalog) == 0 {
		return nil
	}
	current, _ := readStacks(r.Repo())
	selected := splitCSV(current)
	opts := make([]huh.Option[string], 0, len(catalog))
	for _, id := range catalog {
		opts = append(opts, huh.NewOption(id, id).Selected(contains(selected, id)))
	}
	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Опциональные стеки (node + python + go уже в базе)").
			Options(opts...).Value(&chosen),
	)).WithWidth(w).WithHeight(h)
	apply := func() tea.Cmd {
		return func() tea.Msg {
			if err := writeStacks(r.Repo(), joinCSV(chosen)); err != nil {
				return backToMenuMsg{notice: "стеки: " + err.Error()}
			}
			return backToMenuMsg{}
		}
	}
	return &formScreen{form: form, apply: apply}
}
