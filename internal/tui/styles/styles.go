package styles

import "charm.land/lipgloss/v2"

var (
	Title     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	Hint      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	Off       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	FailMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	SelTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	NormTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	Header       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	HeaderRight  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	MenuSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	MenuNormal   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	MenuDisabled = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	MainBorder   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	CmdlogDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Degraded     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	OK           = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	Spinner      = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)
