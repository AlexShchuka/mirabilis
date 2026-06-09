package app

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
	"github.com/AlexShchuka/mirabilis/internal/ui"
)

type launchReadyMsg struct{}

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

func newLaunchForm(r runner.Runner, w, h int) *formScreen {
	repo := r.Repo()
	stackCatalog := config.ReadStackCatalog(repo)
	pluginCatalog := config.ReadPluginCatalog(repo)
	if len(stackCatalog) == 0 && len(pluginCatalog) == 0 {
		return nil
	}

	currentStacks := splitCSV(func() string { v, _ := config.ReadStacks(repo); return v }())
	disabledPlugins := config.ReadPluginsDisabled(repo)

	var chosenStacks []string
	var chosenPlugins []string

	var groups []*huh.Group

	if len(stackCatalog) > 0 {
		opts := make([]huh.Option[string], 0, len(stackCatalog))
		for _, id := range stackCatalog {
			opts = append(opts, huh.NewOption(id, id).Selected(contains(currentStacks, id)))
		}
		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(ui.FormTitleStacks).
				Options(opts...).Value(&chosenStacks),
		))
	}

	if len(pluginCatalog) > 0 {
		opts := make([]huh.Option[string], 0, len(pluginCatalog))
		for _, p := range pluginCatalog {
			opts = append(opts, huh.NewOption(p, p).Selected(!contains(disabledPlugins, p)))
		}
		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(ui.FormTitlePlugins).
				Options(opts...).Value(&chosenPlugins),
		))
	}

	form := huh.NewForm(groups...).WithWidth(w).WithHeight(h)

	apply := func() tea.Cmd {
		return func() tea.Msg {
			if len(stackCatalog) > 0 {
				if err := config.WriteStacks(repo, joinCSV(chosenStacks)); err != nil {
					return backToMenuMsg{notice: ui.NoticeStacksErr + err.Error()}
				}
			}
			if len(pluginCatalog) > 0 {
				var newDisabled []string
				for _, p := range pluginCatalog {
					if !contains(chosenPlugins, p) {
						newDisabled = append(newDisabled, p)
					}
				}
				if err := config.WritePluginsDisabled(repo, newDisabled); err != nil {
					return backToMenuMsg{notice: ui.NoticePluginsErr + err.Error()}
				}
			}
			return launchReadyMsg{}
		}
	}
	return &formScreen{form: form, apply: apply}
}

func applyHarness(ctx context.Context, r runner.Runner, choice string) error {
	switch choice {
	case "off":
		_, err := r.Container(ctx, "bash", "-lc", `echo skip > "$HOME/.claude/.mirabilis-harness"`)
		return err
	case "on":
		_, err := r.Container(ctx, "bash", "-lc", `echo install > "$HOME/.claude/.mirabilis-harness"`)
		return err
	case "reinstall":
		if _, err := r.Container(ctx, "bash", "-lc", `echo install > "$HOME/.claude/.mirabilis-harness"`); err != nil {
			return err
		}
		return provision.EnsureHarness(ctx, r)
	}
	return nil
}

func resetAllCmd(ctx context.Context, r runner.Runner) tea.Cmd {
	return func() tea.Msg {
		if err := runtime.ResetAll(ctx, r); err != nil {
			return backToMenuMsg{notice: ui.NoticeResetErr + err.Error()}
		}
		return backToMenuMsg{notice: ui.NoticeResetDone}
	}
}

func doVSCodeCmd(ctx context.Context, r runner.Runner) tea.Cmd {
	return func() tea.Msg {
		if err := runtime.DoVSCode(ctx, r); err != nil {
			return backToMenuMsg{notice: ui.NoticeVSCodeErr + err.Error()}
		}
		return backToMenuMsg{}
	}
}

func newHarnessForm(ctx context.Context, r runner.Runner, w, h int) *formScreen {
	current := "on"
	if pref, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`); strings.TrimSpace(pref) == "skip" {
		current = "off"
	}
	choice := current
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(ui.FormTitleHarness).
			Options(
				huh.NewOption(ui.FormOptHarnessOn, "on"),
				huh.NewOption(ui.FormOptHarnessOff, "off"),
				huh.NewOption(ui.FormOptHarnessRe, "reinstall"),
			).Value(&choice),
	)).WithWidth(w).WithHeight(h)
	apply := func() tea.Cmd {
		return func() tea.Msg {
			if err := applyHarness(ctx, r, choice); err != nil {
				return backToMenuMsg{notice: ui.NoticeHarnessErr + err.Error()}
			}
			return backToMenuMsg{}
		}
	}
	return &formScreen{form: form, apply: apply}
}

func newResetForm(ctx context.Context, r runner.Runner, w, h int) *formScreen {
	var confirmed bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(ui.FormTitleReset).
			Description(ui.FormDescReset).
			Affirmative(ui.FormConfirmReset).
			Negative(ui.FormCancelReset).
			Value(&confirmed),
	)).WithWidth(w).WithHeight(h)
	apply := func() tea.Cmd {
		if !confirmed {
			return emit(backToMenuMsg{})
		}
		return resetAllCmd(ctx, r)
	}
	return &formScreen{form: form, apply: apply}
}
