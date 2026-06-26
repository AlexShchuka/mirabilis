package screens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func TestRestartWarnID(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	if w.ID() != "app/launch/restart" {
		t.Errorf("ID() = %q", w.ID())
	}
	if w.Init() != nil {
		t.Error("Init() should be nil")
	}
}

func TestRestartWarnDefaultCursorIsCancel(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	if w.options[w.cursor].key != RestartCancel {
		t.Errorf("default option = %q, want %q (safe non-destructive default)", w.options[w.cursor].key, RestartCancel)
	}
}

func TestRestartWarnEnterOnDefaultEmitsCancel(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	_, cmd := w.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	res, ok := emit(cmd).(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", emit(cmd))
	}
	if res.Value != RestartCancel {
		t.Errorf("default enter Value = %v, want %q", res.Value, RestartCancel)
	}
}

func TestRestartWarnDownThenEnterConfirms(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	scr, _ := w.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	w = scr.(RestartWarn)
	_, cmd := w.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	res, ok := emit(cmd).(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", emit(cmd))
	}
	if res.Value != RestartConfirm {
		t.Errorf("Value = %v, want %q", res.Value, RestartConfirm)
	}
}

func TestRestartWarnEscEmitsCancel(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	scr, _ := w.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	w = scr.(RestartWarn)
	_, cmd := w.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	res, ok := emit(cmd).(bus.ScreenResult)
	if !ok {
		t.Fatalf("esc: got %T, want bus.ScreenResult", emit(cmd))
	}
	if res.Value != RestartCancel {
		t.Errorf("esc Value = %v, want %q (esc cancels)", res.Value, RestartCancel)
	}
}

func TestRestartWarnNavigationClamps(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	scr, _ := w.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if scr.(RestartWarn).cursor != 0 {
		t.Errorf("up at top: cursor = %d, want 0 (clamped)", scr.(RestartWarn).cursor)
	}
	w = scr.(RestartWarn)
	for range w.options {
		scr, _ = w.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		w = scr.(RestartWarn)
	}
	if w.cursor != len(w.options)-1 {
		t.Errorf("down past end: cursor = %d, want %d (clamped)", w.cursor, len(w.options)-1)
	}
}

func TestRestartWarnEnvelopeUnwrap(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	_, cmd := w.Update(bus.Envelope{To: "app/launch/restart", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	if _, ok := emit(cmd).(bus.ScreenResult); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenResult", emit(cmd))
	}
}

func TestRestartWarnViewSurfacesLostAndKept(t *testing.T) {
	w := NewRestartWarn("app/launch/restart")
	view := plain(w.View())
	for _, want := range []string{uistr.RestartWarnTitle, uistr.RestartWarnLost, uistr.RestartWarnKept, uistr.RestartWarnConfirm, uistr.RestartWarnCancel} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
