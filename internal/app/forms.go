package app

import (
	"context"
	"fmt"
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

// startPipelineMsg is emitted when the pre-launch form sequence is complete
// (all catalog + telegram forms accepted or skipped) and the pipeline can start.
type startPipelineMsg struct{}

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
	skillCatalog := config.ReadSkillCatalog(repo)
	if len(stackCatalog) == 0 && len(pluginCatalog) == 0 && len(skillCatalog) == 0 {
		return nil
	}

	currentStacks := splitCSV(func() string { v, _ := config.ReadStacks(repo); return v }())
	disabledPlugins := config.ReadPluginsDisabled(repo)
	currentSkills := splitCSV(func() string { v, _ := config.ReadSkills(repo); return v }())

	var chosenStacks []string
	var chosenPlugins []string
	var chosenSkills []string

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

	if len(skillCatalog) > 0 {
		opts := make([]huh.Option[string], 0, len(skillCatalog))
		for _, s := range skillCatalog {
			opts = append(opts, huh.NewOption(s, s).Selected(contains(currentSkills, s)))
		}
		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(ui.FormTitleSkills).
				Options(opts...).Value(&chosenSkills),
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
			if len(skillCatalog) > 0 {
				if err := config.WriteSkills(repo, joinCSV(chosenSkills)); err != nil {
					return backToMenuMsg{notice: ui.NoticeSkillsErr + err.Error()}
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
		return provision.WriteHarnessChoiceContainer(ctx, r, provision.HarnessSkip)
	case "on":
		return provision.WriteHarnessChoiceContainer(ctx, r, provision.HarnessInstall)
	case "reinstall":
		if err := provision.WriteHarnessChoiceContainer(ctx, r, provision.HarnessInstall); err != nil {
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
	if provision.ReadHarnessChoiceContainer(ctx, r) == provision.HarnessSkip {
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

// newTelegramForm builds the optional Telegram-setup step.
//
// The form shows a Select "Настроить / Пропустить" (default: Пропустить).
// If the user chooses "Настроить", a second group asks for the bot token with
// echo-hidden input so the token is never shown in the TUI.
//
// On apply:
//   - Skip  → emits telegramDoneMsg{} immediately; nothing is written.
//   - Configure → writes the token to the host-side secret file via
//     provision.WriteTelegramToken and emits telegramDoneMsg{}.
//
// Channel-ID auto-detection is intentionally NOT implemented here. The bot
// token is sufficient: after the first message is sent via the running bot,
// cmd/tgsmoke / internal/telegram/inbox can discover the chat ID via
// getUpdates. This form only provisions the credential.
func newTelegramForm(w, h int) *formScreen {
	configure := false
	var token string

	selectGroup := huh.NewGroup(
		huh.NewSelect[bool]().
			Title(ui.FormTitleTelegram).
			Options(
				huh.NewOption(ui.FormOptTelegramSkip, false),
				huh.NewOption(ui.FormOptTelegramConfigure, true),
			).
			Value(&configure),
	)

	tokenGroup := huh.NewGroup(
		huh.NewInput().
			Title(ui.FormTitleTelegramToken).
			Description(ui.FormDescTelegramToken).
			EchoMode(huh.EchoModePassword).
			Value(&token).
			Validate(validateTelegramToken),
	).WithHideFunc(func() bool { return !configure })

	form := huh.NewForm(selectGroup, tokenGroup).WithWidth(w).WithHeight(h)

	apply := func() tea.Cmd {
		return applyTelegramToken(configure, token)
	}
	return &formScreen{form: form, apply: apply}
}

// validateTelegramToken is the huh Validate callback for the token input field.
// Exposed as a package-level function so it can be tested directly.
func validateTelegramToken(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("токен не может быть пустым")
	}
	return nil
}

// applyTelegramToken is the apply-logic for the Telegram form extracted into a
// testable function. configure=false → skip (startPipelineMsg), true → write
// the token and emit startPipelineMsg, or backToMenuMsg on error.
func applyTelegramToken(configure bool, token string) tea.Cmd {
	return func() tea.Msg {
		if !configure {
			return telegramDoneMsg{}
		}
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			return backToMenuMsg{notice: ui.NoticeTelegramErr + "токен пустой"}
		}
		if err := provision.WriteTelegramToken(trimmed); err != nil {
			return backToMenuMsg{notice: ui.NoticeTelegramErr + err.Error()}
		}
		return telegramDoneMsg{}
	}
}

func newClaudeTokenForm(ctx context.Context, r runner.Runner, w, h int) *formScreen {
	configure := false
	var token string

	selectGroup := huh.NewGroup(
		huh.NewSelect[bool]().
			Title(ui.FormTitleClaude).
			Options(
				huh.NewOption(ui.FormOptClaudeSkip, false),
				huh.NewOption(ui.FormOptClaudeConfigure, true),
			).
			Value(&configure),
	)

	tokenGroup := huh.NewGroup(
		huh.NewInput().
			Title(ui.FormTitleClaudeToken).
			Description(ui.FormDescClaudeToken).
			EchoMode(huh.EchoModePassword).
			Value(&token).
			Validate(validateClaudeToken),
	).WithHideFunc(func() bool { return !configure })

	form := huh.NewForm(selectGroup, tokenGroup).WithWidth(w).WithHeight(h)

	apply := func() tea.Cmd {
		return applyClaudeToken(ctx, r, configure, token)
	}
	return &formScreen{form: form, apply: apply}
}

func validateClaudeToken(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("токен не может быть пустым")
	}
	return nil
}

func applyClaudeToken(ctx context.Context, r runner.Runner, configure bool, token string) tea.Cmd {
	return func() tea.Msg {
		if !configure {
			return startPipelineMsg{}
		}
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			return backToMenuMsg{notice: ui.NoticeClaudeErr + "токен пустой"}
		}
		if err := provision.WriteClaudeToken(trimmed); err != nil {
			return backToMenuMsg{notice: ui.NoticeClaudeErr + err.Error()}
		}
		if provision.ClaudeCredentialsConflict(ctx, r) {
			return backToMenuMsg{notice: ui.NoticeClaudeConflict}
		}
		return startPipelineMsg{}
	}
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
