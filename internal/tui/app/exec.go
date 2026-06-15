package app

import tea "charm.land/bubbletea/v2"

func SetExecRunner(fn func(tea.ExecCommand, tea.ExecCallback) tea.Cmd) {
	execRunner = fn
}
