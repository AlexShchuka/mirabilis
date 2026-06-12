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
	if w, h := m.MainSize(); w != 80-frame.MenuWidth-1 || h != 22 {
		t.Errorf("MainSize() = (%d,%d), want (%d,22)", w, h, 80-frame.MenuWidth-1)
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view = plain(m.View("main area"))
	if got := lipgloss.Height(view); got != 30 {
		t.Errorf("height = %d, want 30", got)
	}
	if got := lipgloss.Width(view); got != 100 {
		t.Errorf("width = %d, want 100", got)
	}
	if w, h := m.MainSize(); w != 100-frame.MenuWidth-1 || h != 28 {
		t.Errorf("MainSize() = (%d,%d), want (%d,28)", w, h, 100-frame.MenuWidth-1)
	}
}

func TestHeaderShowsStatusAndDegraded(t *testing.T) {
	m := frame.New("mirabilis", "v1.0.0", items())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.Update(bus.StatusChanged{Snapshot: obs.Snapshot{
		"container": {State: obs.StateOK, Detail: "up"},
		"notify":    {State: obs.StateDegraded},
	}})
	header := strings.Split(plain(m.View("")), "\n")[0]
	for _, want := range []string{"mirabilis", "v1.0.0", "container up", "degraded: notify"} {
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
