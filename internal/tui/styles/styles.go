package styles

import (
	"charm.land/lipgloss/v2"

	huh "charm.land/huh/v2"
)

const (
	colSelected = "#00FFC2"
	colCyan     = "#00F5D4"
	colNormal   = "252"
	colDim      = "244"
	colMuted    = "240"
	colDanger   = "203"
	colOK       = "42"
)

var (
	Title     = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	Hint      = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	Off       = lipgloss.NewStyle().Foreground(lipgloss.Color(colMuted))
	FailMark  = lipgloss.NewStyle().Foreground(lipgloss.Color(colDanger))
	SelTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colSelected))
	NormTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(colNormal))

	Header       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colSelected))
	HeaderRight  = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	MenuSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colSelected))
	MenuNormal   = lipgloss.NewStyle().Foreground(lipgloss.Color(colNormal))
	MenuDisabled = lipgloss.NewStyle().Foreground(lipgloss.Color(colMuted))
	MainBorder   = lipgloss.NewStyle().Foreground(lipgloss.Color(colSelected))
	CmdlogDim    = lipgloss.NewStyle().Foreground(lipgloss.Color(colMuted))
	Degraded     = lipgloss.NewStyle().Foreground(lipgloss.Color(colDanger))
	Danger       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colDanger))
	OK           = lipgloss.NewStyle().Foreground(lipgloss.Color(colOK))
	Spinner      = lipgloss.NewStyle().Foreground(lipgloss.Color(colSelected))

	Overlay = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colSelected)).
		Padding(0, 1)
)

func huhStyles(isDark bool) *huh.Styles {
	s := huh.ThemeBase(isDark)
	selected := lipgloss.Color(colSelected)
	cyan := lipgloss.Color(colCyan)
	normal := lipgloss.Color(colNormal)
	dim := lipgloss.Color(colDim)
	danger := lipgloss.Color(colDanger)

	s.Focused.Title = s.Focused.Title.Foreground(selected).Bold(true)
	s.Focused.Description = s.Focused.Description.Foreground(dim)
	s.Focused.SelectSelector = s.Focused.SelectSelector.Foreground(selected)
	s.Focused.MultiSelectSelector = s.Focused.MultiSelectSelector.Foreground(selected)
	s.Focused.SelectedOption = s.Focused.SelectedOption.Foreground(selected)
	s.Focused.Option = s.Focused.Option.Foreground(normal)
	s.Focused.ErrorIndicator = s.Focused.ErrorIndicator.Foreground(danger)
	s.Focused.ErrorMessage = s.Focused.ErrorMessage.Foreground(danger)
	s.Focused.FocusedButton = s.Focused.FocusedButton.Foreground(cyan)

	s.Blurred.Title = s.Blurred.Title.Foreground(dim)
	s.Blurred.Description = s.Blurred.Description.Foreground(dim)
	s.Blurred.Option = s.Blurred.Option.Foreground(normal)

	s.Group.Title = s.Group.Title.Foreground(selected).Bold(true)
	s.Group.Description = s.Group.Description.Foreground(dim)
	return s
}

func HuhTheme() huh.Theme {
	return huh.ThemeFunc(huhStyles)
}

func HuhThemeDanger() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		s := huhStyles(isDark)
		danger := lipgloss.Color(colDanger)
		s.Focused.FocusedButton = s.Focused.FocusedButton.Background(danger).Foreground(lipgloss.Color("0")).Bold(true)
		s.Focused.Title = s.Focused.Title.Foreground(danger).Bold(true)
		return s
	})
}
