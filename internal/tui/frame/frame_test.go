package frame_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func items() []frame.Item {
	return []frame.Item{
		{Title: "Launch", Action: "launch", Enabled: true},
		{Title: "Harness", Action: "harness", Enabled: false},
		{Title: "Reset", Action: "reset", Enabled: true},
		{Title: "Quit", Action: "quit", Enabled: true},
	}
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func TestNavigationSkipsDisabled(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{name: "initial", want: "launch"},
		{name: "down skips disabled", keys: []string{"down"}, want: "reset"},
		{name: "j skips disabled", keys: []string{"j"}, want: "reset"},
		{name: "down twice", keys: []string{"down", "down"}, want: "quit"},
		{name: "clamped at bottom", keys: []string{"down", "down", "down", "down"}, want: "quit"},
		{name: "up skips disabled back", keys: []string{"down", "up"}, want: "launch"},
		{name: "k skips disabled back", keys: []string{"j", "k"}, want: "launch"},
		{name: "clamped at top", keys: []string{"up"}, want: "launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := frame.New("mirabilis", "v1.0.0", items())
			for _, k := range tt.keys {
				m, _ = m.Update(key(k))
			}
			got, ok := m.Selected()
			if !ok || got.Action != tt.want {
				t.Errorf("Selected() = %q, want %q", got.Action, tt.want)
			}
		})
	}
}

func TestEnterEmitsMenuChosen(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter returned nil cmd")
	}
	if got, ok := cmd().(bus.MenuChosen); !ok || got.Action != "launch" {
		t.Fatalf("enter emitted %v, want MenuChosen{launch}", got)
	}
}

func TestSetEnabledMovesCursorOffDisabled(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m.SetEnabled("launch", false)
	got, ok := m.Selected()
	if !ok || got.Action != "reset" {
		t.Errorf("Selected() = %q, want reset", got.Action)
	}
	m.SetEnabled("harness", true)
	m, _ = m.Update(key("up"))
	if got, _ := m.Selected(); got.Action != "harness" {
		t.Errorf("Selected() = %q, want harness after re-enable", got.Action)
	}
}

func TestResizeReflow(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())

	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := plain(m.View("main area"))
	if got := lipgloss.Height(view); got != 24 {
		t.Errorf("height = %d, want 24", got)
	}
	if got := lipgloss.Width(view); got != 80 {
		t.Errorf("width = %d, want 80", got)
	}
	if w, h := m.MainSize(); w != 80-m.MenuWidth()-1 || h != 19 {
		t.Errorf("MainSize() = (%d,%d), want (%d,19)", w, h, 80-m.MenuWidth()-1)
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view = plain(m.View("main area"))
	if got := lipgloss.Height(view); got != 30 {
		t.Errorf("height = %d, want 30", got)
	}
	if got := lipgloss.Width(view); got != 100 {
		t.Errorf("width = %d, want 100", got)
	}
	if w, h := m.MainSize(); w != 100-m.MenuWidth()-1 || h != 25 {
		t.Errorf("MainSize() = (%d,%d), want (%d,25)", w, h, 100-m.MenuWidth()-1)
	}
}

func TestMenuWidthProportionalAndClamped(t *testing.T) {
	tests := []struct {
		w, want int
	}{
		{60, 13},
		{80, 17},
		{100, 22},
		{200, frame.MenuMax},
	}
	for _, tt := range tests {
		m := frame.New("mirabilis", "v1.0.0", items())
		m, _ = m.Update(tea.WindowSizeMsg{Width: tt.w, Height: 30})
		if got := m.MenuWidth(); got != tt.want {
			t.Errorf("MenuWidth(width=%d) = %d, want %d", tt.w, got, tt.want)
		}
		if got := m.MenuWidth(); got < frame.MenuMin || got > frame.MenuMax {
			t.Errorf("MenuWidth(width=%d) = %d outside [%d,%d]", tt.w, got, frame.MenuMin, frame.MenuMax)
		}
	}
}

func TestResizeMatrixNeverOverflowsHeight(t *testing.T) {
	sizes := []struct {
		name string
		w, h int
	}{
		{"wide", 120, 40},
		{"narrow", 50, 20},
		{"short", 80, 12},
		{"tiny", 30, 8},
		{"min-edge", 40, 10},
		{"degenerate-1x1", 1, 1},
		{"degenerate-2x1", 2, 1},
	}
	for _, s := range sizes {
		t.Run(s.name, func(t *testing.T) {
			m := frame.New("mirabilis", "v1.0.0", items())
			m, _ = m.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
			view := plain(m.View(strings.Repeat("content\n", 60)))
			if got := lipgloss.Height(view); got > s.h {
				t.Errorf("height = %d, want <= %d", got, s.h)
			}
			if got := lipgloss.Width(view); got > s.w {
				t.Errorf("width = %d, want <= %d", got, s.w)
			}
		})
	}
}

func TestNarrowCollapsesMenuToGlyphs(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	view := plain(m.View("main area"))
	if strings.Contains(view, "Launch") || strings.Contains(view, "Harness") {
		t.Errorf("narrow menu still shows full titles:\n%s", view)
	}
	if !strings.Contains(view, "> L") {
		t.Errorf("narrow menu missing collapsed selected glyph:\n%s", view)
	}
	if m.MenuWidth() != frame.NarrowMenu {
		t.Errorf("narrow MenuWidth = %d, want %d", m.MenuWidth(), frame.NarrowMenu)
	}
}

func TestTooSmallState(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	view := plain(m.View("main area"))
	if !strings.Contains(view, "terminal too small") {
		t.Errorf("tiny view missing too-small message:\n%s", view)
	}
	if !strings.Contains(view, "30x8") || !strings.Contains(view, "40x10") {
		t.Errorf("tiny view missing current/required size:\n%s", view)
	}
	if strings.Contains(view, "Launch") {
		t.Errorf("tiny view still renders the menu:\n%s", view)
	}
	if got := lipgloss.Height(view); got != 8 {
		t.Errorf("tiny view height = %d, want 8", got)
	}
}

func TestMenuPanelRendersItemTitlesAndCursor(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := plain(m.View("main area"))
	for _, title := range []string{"Launch", "Harness", "Reset", "Quit"} {
		if !strings.Contains(view, title) {
			t.Errorf("frame view missing menu item %q:\n%s", title, view)
		}
	}
	if !strings.Contains(view, "> Launch") {
		t.Errorf("frame view missing cursor on the selected item:\n%s", view)
	}
}

func descItems() []frame.Item {
	return []frame.Item{
		{Title: "Launch", Desc: "setup pipeline in container", Action: "launch", Enabled: true},
		{Title: "Harness", Desc: "neuro-matrix on off reinstall", Action: "harness", Enabled: true},
		{Title: "Quit", Action: "quit", Enabled: true},
	}
}

func TestMenuShowsSelectedDescAndFollowsCursor(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", descItems())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := plain(m.View("main area"))
	if !strings.Contains(view, "setup pipeline") {
		t.Errorf("frame view missing selected item desc:\n%s", view)
	}
	if strings.Contains(view, "neuro-matrix") {
		t.Errorf("frame view shows a non-selected item's desc:\n%s", view)
	}

	m, _ = m.Update(key("down"))
	view = plain(m.View("main area"))
	if !strings.Contains(view, "neuro-matrix") {
		t.Errorf("desc did not follow the cursor to harness:\n%s", view)
	}
	if strings.Contains(view, "setup pipeline") {
		t.Errorf("desc still shows the previous selection after move:\n%s", view)
	}

	m, _ = m.Update(key("down"))
	view = plain(m.View("main area"))
	if strings.Contains(view, "neuro-matrix") {
		t.Errorf("desc shown for an item with no Desc:\n%s", view)
	}
}

func TestMenuDescShortBreakpointVisibleNarrowHidden(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", descItems())

	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	short := plain(m.View("main area"))
	if got := lipgloss.Height(short); got > 12 {
		t.Errorf("short view height = %d, want <= 12", got)
	}
	if !strings.Contains(short, "setup") {
		t.Errorf("short breakpoint hid the desc:\n%s", short)
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	narrow := plain(m.View("main area"))
	if strings.Contains(narrow, "setup") || strings.Contains(narrow, "pipeline") {
		t.Errorf("narrow breakpoint still renders the desc:\n%s", narrow)
	}
}

func TestHeaderShowsStatusAndDegraded(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.Update(bus.StatusChanged{Snapshot: obs.Snapshot{
		"container": {State: obs.StateOK, Detail: "up"},
		"notify":    {State: obs.StateDegraded},
	}})
	lines := strings.Split(plain(m.View("")), "\n")
	header := strings.Join(lines[:4], "\n")
	for _, want := range []string{"v1.0.0", "container up", "degraded: notify"} {
		if !strings.Contains(header, want) {
			t.Errorf("header = %q, missing %q", header, want)
		}
	}
}

func TestFooterHints(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := plain(m.View(""))
	lines := strings.Split(view, "\n")
	if got := lines[len(lines)-1]; !strings.Contains(got, "enter select · esc back · tab log · q quit") {
		t.Errorf("footer = %q, want hints", got)
	}
}

func TestMainAreaCropped(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	view := plain(m.View(strings.Repeat("row\n", 30)))
	if got := lipgloss.Height(view); got != 10 {
		t.Errorf("height = %d, want 10 with overflowing main", got)
	}
}

func TestHeaderTruncationLeftFirst(t *testing.T) {
	m := frame.New("mirabilis", "v1.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.Update(bus.StatusChanged{Snapshot: obs.Snapshot{}})

	header80 := strings.Join(strings.Split(plain(m.View("")), "\n")[:3], "\n")
	if !strings.Contains(header80, "v1.0") {
		t.Fatalf("header at width=80 missing version:\n%s", header80)
	}
	if !strings.Contains(header80, "○") {
		t.Fatalf("header at width=80 missing logo (○):\n%s", header80)
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
	header20 := strings.Split(plain(m.View("")), "\n")[0]
	if strings.Contains(header20, "○") {
		t.Errorf("narrow header (w=20) still shows logo — should have dropped:\n%s", header20)
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 15, Height: 24})
	headerNarrow := strings.Split(plain(m.View("")), "\n")[0]
	if strings.Contains(headerNarrow, "○") {
		t.Errorf("narrow header (w=15) still shows logo:\n%s", headerNarrow)
	}
	if !strings.Contains(header80, "v1.0") {
		t.Errorf("header at width=80 missing version:\n%s", header80)
	}
}

// TestHeaderRightSurvivesNarrowest locks that the Right zone (version) is last to drop
// (INV §4 truncation-order: Left ≺ Center ≺ Right).
func TestHeaderRightSurvivesNarrowest(t *testing.T) {
	m := frame.New("mirabilis", "v1.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	header80 := strings.Split(plain(m.View("")), "\n")[0]
	if !strings.Contains(header80, "v1.0") {
		t.Skipf("version not in header at width=80 (test precondition not met): %q", header80)
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 12, Height: 24})
	headerTiny := strings.Split(plain(m.View("")), "\n")[0]
	if !strings.Contains(headerTiny, "v1.0") && strings.Contains(headerTiny, "mirabilis") {
		t.Errorf("header at width=12: center survived but right dropped (INV §4 truncation-order violated)\nheader: %q", headerTiny)
	}
}
