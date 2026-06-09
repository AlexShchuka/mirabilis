package ui

import "charm.land/lipgloss/v2"

var (
	TitleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	HintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	OffStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	FailMarkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	SelTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	NormTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)
