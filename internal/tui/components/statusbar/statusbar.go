package statusbar

import (
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type Model struct {
	snap obs.Snapshot
}

func New() Model {
	return Model{}
}

func (m Model) Update(msg tea.Msg) Model {
	if sc, ok := msg.(bus.StatusChanged); ok {
		m.snap = sc.Snapshot
	}
	return m
}

func (m Model) View() string {
	if len(m.snap) == 0 {
		return ""
	}
	nodes := slices.Sorted(maps.Keys(m.snap))
	segments := make([]string, 0, len(nodes)+1)
	var degraded []string
	for _, node := range nodes {
		st := m.snap[node]
		if st.State == obs.StateDegraded {
			degraded = append(degraded, node)
			continue
		}
		seg := node + " "
		if st.Detail != "" {
			seg += st.Detail
		} else {
			seg += st.State.String()
		}
		style := styles.HeaderRight
		if st.State == obs.StateOK {
			style = styles.OK
		}
		segments = append(segments, style.Render(seg))
	}
	if len(degraded) > 0 {
		segments = append(segments, styles.Degraded.Render(uistr.DegradedPrefix+strings.Join(degraded, ", ")))
	}
	return strings.Join(segments, uistr.StatusSep)
}
