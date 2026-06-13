package steplist

import (
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/a11y"
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

const (
	progressFPS      = 60
	progressBarWidth = 20
	springFreq       = 9.0
	springDamp       = 0.7
)

type progressTickMsg struct{}

func progressTick() tea.Cmd {
	return tea.Tick(time.Second/progressFPS, func(time.Time) tea.Msg { return progressTickMsg{} })
}

type Model struct {
	spin    spinner.Model
	rows    []StepRow
	width   int
	height  int
	spring  harmonica.Spring
	pos     float64
	vel     float64
	target  float64
	animate bool
}

func New(rows []StepRow) Model {
	m := Model{
		spin:   spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styles.Spinner)),
		rows:   append([]StepRow(nil), rows...),
		spring: harmonica.NewSpring(harmonica.FPS(progressFPS), springFreq, springDamp),
	}
	m.target = m.ratio()
	m.pos = m.target
	return m
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m Model) Init() tea.Cmd {
	if a11y.ReducedMotion() {
		return nil
	}
	return m.spin.Tick
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.StepEvent:
		return m.apply(msg)
	case spinner.TickMsg:
		if a11y.ReducedMotion() || !m.anyRunning() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case progressTickMsg:
		return m.advanceProgress()
	}
	return m, nil
}

func (m Model) advanceProgress() (Model, tea.Cmd) {
	if !m.animate {
		return m, nil
	}
	m.pos, m.vel = m.spring.Update(m.pos, m.vel, m.target)
	if absf(m.target-m.pos) < 0.001 && absf(m.vel) < 0.001 {
		m.pos = m.target
		m.vel = 0
		m.animate = false
		return m, nil
	}
	return m, progressTick()
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func (m *Model) retarget() tea.Cmd {
	m.target = m.ratio()
	if a11y.ReducedMotion() {
		m.pos = m.target
		m.vel = 0
		m.animate = false
		return nil
	}
	if m.animate {
		return nil
	}
	m.animate = true
	return progressTick()
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
	switch ev.Kind {
	case bus.StepStarted:
		if a11y.ReducedMotion() {
			return m, nil
		}
		return m, m.spin.Tick
	case bus.StepDone, bus.StepFailed, bus.StepSkipped:
		return m, m.retarget()
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

func (m Model) completed() int {
	n := 0
	for _, r := range m.rows {
		switch r.State {
		case StateDone, StateFailed, StateSkipped:
			n++
		}
	}
	return n
}

func (m Model) ratio() float64 {
	if len(m.rows) == 0 {
		return 0
	}
	return float64(m.completed()) / float64(len(m.rows))
}

func (m Model) View() string {
	lines := make([]string, 0, len(m.rows)+1)
	if bar := m.progressView(); bar != "" {
		lines = append(lines, bar)
	}
	for _, r := range m.rows {
		line := " " + m.glyph(r) + " " + styles.NormTitle.Width(titleWidth).Render(r.Title)
		if r.Detail != "" {
			line += " " + styles.Hint.Render(r.Detail)
		}
		if m.width > 0 {
			line = truncate(line, m.width)
		}
		lines = append(lines, line)
	}
	lines = m.clampHeight(lines)
	return strings.Join(lines, "\n")
}

func (m Model) progressView() string {
	done := m.completed()
	total := len(m.rows)
	if total == 0 || done == 0 {
		return ""
	}
	fill := int(m.pos*progressBarWidth + 0.5)
	if fill < 0 {
		fill = 0
	}
	if fill > progressBarWidth {
		fill = progressBarWidth
	}
	bar := styles.OK.Render(strings.Repeat("█", fill)) + styles.Off.Render(strings.Repeat("░", progressBarWidth-fill))
	count := strconv.Itoa(done) + uistr.ProgressSep + strconv.Itoa(total)
	return " " + bar + " " + styles.Hint.Render(count)
}

func (m Model) clampHeight(lines []string) []string {
	if m.height <= 0 || len(lines) <= m.height {
		return lines
	}
	keep := max(m.height-1, 1)
	hidden := len(lines) - keep
	out := lines[:keep]
	overflow := uistr.StepOverflowPrefix + strconv.Itoa(hidden) + uistr.StepOverflowSuffix
	if m.width > 0 {
		overflow = truncate(overflow, m.width)
	}
	return append(out, styles.Off.Render(overflow))
}

func truncate(s string, w int) string {
	if w <= 0 || lipgloss.Width(s) <= w {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}

func (m Model) glyph(r StepRow) string {
	switch r.State {
	case StateRunning:
		if a11y.ReducedMotion() {
			return styles.Spinner.Render(uistr.GlyphRunningStatic)
		}
		return m.spin.View()
	case StateDone:
		return styles.OK.Render(uistr.GlyphDone)
	case StateFailed:
		return styles.FailMark.Render(uistr.GlyphFailed)
	case StateSkipped:
		return styles.Off.Render(uistr.GlyphSkipped)
	case StateWaiting:
		return styles.Spinner.Render(uistr.GlyphWaiting)
	default:
		return styles.Off.Render(uistr.GlyphPending)
	}
}
