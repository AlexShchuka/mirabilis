package steplist

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type State int

const (
	StatePending State = iota
	StateRunning
	StateDone
	StateFailed
	StateSkipped
	StateWaiting
)

type StepRow struct {
	Name   string
	Title  string
	Detail string
	State  State
}

const titleWidth = 17

type Model struct {
	spin spinner.Model
	rows []StepRow
}

func New(rows []StepRow) Model {
	return Model{
		spin: spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styles.Spinner)),
		rows: append([]StepRow(nil), rows...),
	}
}

func (m Model) Init() tea.Cmd {
	return m.spin.Tick
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.StepEvent:
		return m.apply(msg)
	case spinner.TickMsg:
		if !m.anyRunning() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) apply(ev bus.StepEvent) (Model, tea.Cmd) {
	rows := append([]StepRow(nil), m.rows...)
	m.rows = rows
	for i := range rows {
		if rows[i].Name != ev.Step {
			continue
		}
		switch ev.Kind {
		case bus.StepStarted:
			rows[i].State = StateRunning
			rows[i].Detail = ev.Line
		case bus.StepLine:
			rows[i].Detail = ev.Line
		case bus.StepDone:
			rows[i].State = StateDone
			setDetail(&rows[i], ev.Line)
		case bus.StepFailed:
			rows[i].State = StateFailed
			setDetail(&rows[i], ev.Line)
		case bus.StepSkipped:
			rows[i].State = StateSkipped
			setDetail(&rows[i], ev.Line)
		case bus.StepWaiting:
			rows[i].State = StateWaiting
			rows[i].Detail = ev.Line
			if rows[i].Detail == "" {
				rows[i].Detail = uistr.StepDetailWaiting
			}
		}
	}
	if ev.Kind == bus.StepStarted {
		return m, m.spin.Tick
	}
	return m, nil
}

func setDetail(row *StepRow, line string) {
	if line != "" {
		row.Detail = line
	}
}

func (m Model) anyRunning() bool {
	for _, r := range m.rows {
		if r.State == StateRunning {
			return true
		}
	}
	return false
}

func (m Model) Rows() []StepRow {
	return append([]StepRow(nil), m.rows...)
}

func (m Model) View() string {
	lines := make([]string, 0, len(m.rows))
	for _, r := range m.rows {
		line := " " + m.glyph(r) + " " + styles.NormTitle.Width(titleWidth).Render(r.Title)
		if r.Detail != "" {
			line += " " + styles.Hint.Render(r.Detail)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m Model) glyph(r StepRow) string {
	switch r.State {
	case StateRunning:
		return m.spin.View()
	case StateDone:
		return styles.OK.Render(uistr.GlyphDone)
	case StateFailed:
		return styles.FailMark.Render(uistr.GlyphFailed)
	case StateSkipped:
		return styles.Off.Render(uistr.GlyphSkipped)
	default:
		return styles.Off.Render(uistr.GlyphPending)
	}
}
