// Package statusbar is a TUI status-bar component showing subsystem health snapshots.
package statusbar

import (
	"maps"
	"slices"
	"strconv"
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

func (m Model) Health() string {
	var ok, degraded, off, unknown int
	for node, st := range m.snap {
		if node == uistr.VersionNode {
			continue
		}
		switch st.State {
		case obs.StateOK:
			ok++
		case obs.StateDegraded:
			degraded++
		case obs.StateOff:
			off++
		default:
			unknown++
		}
	}
	if ok+degraded+off+unknown == 0 {
		return ""
	}
	parts := make([]string, 0, 4)
	if ok > 0 {
		parts = append(parts, styles.OK.Render(uistr.GlyphStatusOK+uistr.HealthCountSep+strconv.Itoa(ok)))
	}
	if degraded > 0 {
		parts = append(parts, styles.Degraded.Render(uistr.GlyphStatusDegraded+uistr.HealthCountSep+strconv.Itoa(degraded)))
	}
	if off > 0 {
		parts = append(parts, styles.Off.Render(uistr.GlyphStatusOff+uistr.HealthCountSep+strconv.Itoa(off)))
	}
	if unknown > 0 {
		parts = append(parts, styles.HeaderRight.Render(uistr.GlyphStatusUnknown+uistr.HealthCountSep+strconv.Itoa(unknown)))
	}
	return strings.Join(parts, uistr.HealthSep)
}

func (m Model) View() string {
	if len(m.snap) == 0 {
		return ""
	}
	nodes := slices.Sorted(maps.Keys(m.snap))
	segments := make([]string, 0, len(nodes)+1)
	var degraded []string
	for _, node := range nodes {
		if node == uistr.VersionNode {
			continue
		}
		st := m.snap[node]
		if st.State == obs.StateDegraded {
			degraded = append(degraded, node)
			continue
		}
		seg := statusGlyph(st.State) + " " + node + " "
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
		seg := statusGlyph(obs.StateDegraded) + " " + uistr.DegradedPrefix + strings.Join(degraded, ", ")
		segments = append(segments, styles.Degraded.Render(seg))
	}
	return strings.Join(segments, uistr.StatusSep)
}

func statusGlyph(s obs.State) string {
	switch s {
	case obs.StateOK:
		return uistr.GlyphStatusOK
	case obs.StateDegraded:
		return uistr.GlyphStatusDegraded
	case obs.StateOff:
		return uistr.GlyphStatusOff
	default:
		return uistr.GlyphStatusUnknown
	}
}
