package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/cmdlog"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/steplist"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
)

func (a App) backToMenu(notice string) (tea.Model, tea.Cmd) {
	if !a.busy {
		a.frame.SetBusy("")
	}
	menu := screens.NewMenu("app/menu")
	if notice != "" {
		menu = menu.WithNotice(notice)
	}
	if a.errNotice != "" {
		menu = menu.WithError(a.errNotice)
	}
	a.router = router.New(menu)
	return a, nil
}

func (a App) failToMenu(notice string) (tea.Model, tea.Cmd) {
	a.errNotice = notice
	return a.backToMenu(notice)
}

func stepsToRows(cmds []pipeline.Command) []steplist.StepRow {
	rows := make([]steplist.StepRow, 0, len(cmds))
	for _, c := range cmds {
		m := c.Meta()
		rows = append(rows, steplist.StepRow{Name: m.Name, Title: m.Title})
	}
	return rows
}

func launchScreen(rows []steplist.StepRow) launchScr {
	sl := steplist.New(rows)
	cl := cmdlog.New()
	return launchScr{
		id:       "app/launch",
		steps:    sl,
		cmdlog:   cl,
		tabFocus: false,
	}
}
