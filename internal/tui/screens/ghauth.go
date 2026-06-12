package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type GHAuth struct {
	id    bus.NodeID
	code  string
	url   string
	lines []string
	done  bool
}

func NewGHAuth(id bus.NodeID, code, url string) GHAuth {
	return GHAuth{
		id:    id,
		code:  code,
		url:   url,
		lines: []string{uistr.GHAuthStatusWaiting},
	}
}

func (g GHAuth) ID() bus.NodeID { return g.id }

func (g GHAuth) Init() tea.Cmd { return nil }

func (g GHAuth) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.Envelope:
		return g.Update(msg.Msg)
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			g.done = true
			return g, func() tea.Msg { return bus.ScreenPop{} }
		}
	case bus.StepEvent:
		if msg.Kind == bus.StepLine && msg.Line != "" {
			g.lines = append(g.lines, msg.Line)
		}
	}
	return g, nil
}

func (g GHAuth) View() string {
	var b strings.Builder
	b.WriteString(" " + styles.Title.Render(uistr.GHAuthTitle) + "\n\n")
	b.WriteString(" " + styles.NormTitle.Render(uistr.GHAuthLabelCode) + g.code + "\n")
	b.WriteString(" " + styles.NormTitle.Render(uistr.GHAuthLabelURL) + g.url + "\n\n")
	for _, l := range g.lines {
		b.WriteString(" " + styles.Hint.Render(l) + "\n")
	}
	return b.String()
}
