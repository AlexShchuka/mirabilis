package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/cmdlog"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/form"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/steplist"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
)

type toggleCmdlogMsg struct{}

type launchScr struct {
	steps    steplist.Model
	cmdlog   cmdlog.Model
	id       bus.NodeID
	tabFocus bool
}

var _ router.Screen = launchScr{}

func (s launchScr) ID() bus.NodeID { return s.id }

func (s launchScr) Init() tea.Cmd {
	return s.steps.Init()
}

func (s launchScr) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mw := msg.Width
		mh := msg.Height
		s.cmdlog.SetSize(mw, mh/3)
		return s, nil
	case toggleCmdlogMsg:
		if s.tabFocus {
			s.cmdlog.Blur()
			s.tabFocus = false
		} else {
			s.cmdlog.Focus()
			s.tabFocus = true
		}
		return s, nil
	case bus.StepEvent:
		var sc, cc tea.Cmd
		s.steps, sc = s.steps.Update(msg)
		s.cmdlog, cc = s.cmdlog.Update(msg)
		return s, tea.Batch(sc, cc)
	case bus.Envelope:
		return s.Update(msg.Msg)
	case tea.KeyPressMsg:
		if s.tabFocus {
			var cc tea.Cmd
			s.cmdlog, cc = s.cmdlog.Update(msg)
			return s, cc
		}
		return s, nil
	}
	var sc tea.Cmd
	s.steps, sc = s.steps.Update(msg)
	return s, sc
}

func (s launchScr) View() string {
	return s.steps.View() + "\n" + s.cmdlog.View()
}

type formScreen struct {
	form form.Model
	id   bus.NodeID
}

var _ router.Screen = formScreen{}

func (s formScreen) ID() bus.NodeID { return s.id }

func (s formScreen) Init() tea.Cmd { return s.form.Init() }

func (s formScreen) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.Envelope:
		return s.Update(msg.Msg)
	case tea.WindowSizeMsg:
		s.form.SetSize(msg.Width, msg.Height)
		return s, nil
	}
	var cmd tea.Cmd
	s.form, cmd = s.form.Update(msg)
	return s, cmd
}

func (s formScreen) View() string { return s.form.View() }
