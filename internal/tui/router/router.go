package router

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
)

type Screen interface {
	ID() bus.NodeID
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View() string
}

type Model struct {
	stack []Screen
}

func New(root Screen) Model {
	return Model{stack: []Screen{root}}
}

func (m Model) Init() tea.Cmd {
	return m.stack[0].Init()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.ScreenPush:
		s, ok := msg.Model.(Screen)
		if !ok {
			return m, nil
		}
		m.stack = append(m.stack[:len(m.stack):len(m.stack)], s)
		return m, s.Init()
	case bus.ScreenPop:
		if len(m.stack) > 1 {
			m.stack = m.stack[: len(m.stack)-1 : len(m.stack)-1]
		}
		return m, nil
	case bus.Envelope:
		if msg.To == "" {
			return m.broadcast(msg.Msg)
		}
		return m.address(msg)
	case tea.WindowSizeMsg:
		return m.broadcast(msg)
	}
	return m.updateTop(msg)
}

func (m Model) address(env bus.Envelope) (Model, tea.Cmd) {
	stack := append([]Screen(nil), m.stack...)
	m.stack = stack
	for i := len(stack) - 1; i >= 0; i-- {
		if !stack[i].ID().Contains(env.To) {
			continue
		}
		var cmd tea.Cmd
		stack[i], cmd = stack[i].Update(env)
		return m, cmd
	}
	return m, nil
}

func (m Model) broadcast(msg tea.Msg) (Model, tea.Cmd) {
	stack := append([]Screen(nil), m.stack...)
	m.stack = stack
	cmds := make([]tea.Cmd, 0, len(stack))
	for i := range stack {
		var cmd tea.Cmd
		stack[i], cmd = stack[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) updateTop(msg tea.Msg) (Model, tea.Cmd) {
	stack := append([]Screen(nil), m.stack...)
	m.stack = stack
	top := len(stack) - 1
	var cmd tea.Cmd
	stack[top], cmd = stack[top].Update(msg)
	return m, cmd
}

func (m Model) Top() Screen {
	return m.stack[len(m.stack)-1]
}

func (m Model) Below() Screen {
	if len(m.stack) < 2 {
		return m.stack[0]
	}
	return m.stack[len(m.stack)-2]
}

func (m Model) Depth() int {
	return len(m.stack)
}

func (m Model) View() string {
	return m.Top().View()
}
