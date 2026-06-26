package screens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func key(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func TestTuneID(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", true)
	if tn.ID() != "app/launch/tune" {
		t.Errorf("ID() = %q", tn.ID())
	}
	if tn.Init() != nil {
		t.Error("Init() should be nil")
	}
}

func TestTunePrefillsEffortAndFleet(t *testing.T) {
	tn := NewTune("app/launch/tune", "xhigh", true)
	if tn.effort != "xhigh" {
		t.Errorf("effort = %q, want xhigh", tn.effort)
	}
	if !tn.fleet {
		t.Error("fleet = false, want true (pre-filled on)")
	}
}

func TestTuneNormalizesUnknownEffort(t *testing.T) {
	tn := NewTune("app/launch/tune", "", false)
	if tn.effort != uistr.EffortMedium {
		t.Errorf("empty effort normalized to %q, want %q", tn.effort, uistr.EffortMedium)
	}
	tn = NewTune("app/launch/tune", "bogus", false)
	if tn.effort != uistr.EffortMedium {
		t.Errorf("unknown effort normalized to %q, want %q", tn.effort, uistr.EffortMedium)
	}
}

func TestTuneEffortCyclesRight(t *testing.T) {
	tn := NewTune("app/launch/tune", "low", false)
	want := []string{"medium", "high", "xhigh", "max", "max"}
	for i, w := range want {
		scr, _ := tn.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		tn = scr.(Tune)
		if tn.effort != w {
			t.Errorf("after %d right: effort = %q, want %q", i+1, tn.effort, w)
		}
	}
}

func TestTuneEffortCyclesLeftClampsAtLow(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", false)
	want := []string{"medium", "low", "low"}
	for i, w := range want {
		scr, _ := tn.Update(key("h"))
		tn = scr.(Tune)
		if tn.effort != w {
			t.Errorf("after %d left: effort = %q, want %q", i+1, tn.effort, w)
		}
	}
}

func TestTuneFleetToggleOnSecondRow(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", false)
	scr, _ := tn.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tn = scr.(Tune)
	if tn.row != tuneRowFleet {
		t.Fatalf("row = %d after down, want fleet row %d", tn.row, tuneRowFleet)
	}
	scr, _ = tn.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	tn = scr.(Tune)
	if !tn.fleet {
		t.Error("fleet still off after toggle on fleet row")
	}
	scr, _ = tn.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	tn = scr.(Tune)
	if tn.fleet {
		t.Error("fleet still on after second toggle (space)")
	}
}

func TestTuneAdjustOnFleetRowDoesNotChangeEffort(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", false)
	scr, _ := tn.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tn = scr.(Tune)
	scr, _ = tn.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	tn = scr.(Tune)
	if tn.effort != "high" {
		t.Errorf("effort changed to %q while on fleet row, want high", tn.effort)
	}
}

func TestTuneRowClampsAtBounds(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", false)
	scr, _ := tn.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	tn = scr.(Tune)
	if tn.row != tuneRowEffort {
		t.Errorf("up at top: row = %d, want 0", tn.row)
	}
	scr, _ = tn.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tn = scr.(Tune)
	scr, _ = tn.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tn = scr.(Tune)
	if tn.row != tuneRowFleet {
		t.Errorf("down past bottom: row = %d, want %d (clamped)", tn.row, tuneRowFleet)
	}
}

func TestTuneEnterEmitsResult(t *testing.T) {
	tn := NewTune("app/launch/tune", "low", false)
	scr, _ := tn.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	tn = scr.(Tune)
	scr, _ = tn.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tn = scr.(Tune)
	scr, _ = tn.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	tn = scr.(Tune)
	_, cmd := tn.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := emit(cmd)
	res, ok := msg.(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", msg)
	}
	tr, ok := res.Value.(TuneResult)
	if !ok {
		t.Fatalf("ScreenResult.Value = %T, want TuneResult", res.Value)
	}
	if tr.Effort != "medium" || !tr.Fleet {
		t.Errorf("TuneResult = %+v, want {medium true}", tr)
	}
}

func TestTuneEscEmitsScreenPop(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", false)
	_, cmd := tn.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := emit(cmd).(bus.ScreenPop); !ok {
		t.Fatalf("esc: got %T, want bus.ScreenPop", emit(cmd))
	}
}

func TestTuneEnvelopeUnwrap(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", false)
	_, cmd := tn.Update(bus.Envelope{To: "app/launch/tune", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	if _, ok := emit(cmd).(bus.ScreenPop); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenPop", emit(cmd))
	}
}

func TestTuneOtherKeysIgnored(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", false)
	scr, cmd := tn.Update(key("x"))
	if cmd != nil {
		t.Error("x key produced a command, want nil")
	}
	if got := scr.(Tune); got.effort != "high" || got.row != 0 {
		t.Errorf("x key mutated state: %+v", got)
	}
}

func TestTuneViewRendersBothRowsAndValues(t *testing.T) {
	tn := NewTune("app/launch/tune", "xhigh", true)
	view := plain(tn.View())
	for _, want := range []string{
		uistr.TuneTitle,
		uistr.TuneEffortLabel,
		uistr.TuneFleetLabel,
		"xhigh",
		uistr.TuneFleetOn,
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestTuneViewShowsFleetOffWhenSolo(t *testing.T) {
	tn := NewTune("app/launch/tune", "medium", false)
	view := plain(tn.View())
	if !strings.Contains(view, uistr.TuneFleetOff) {
		t.Errorf("view missing fleet off:\n%s", view)
	}
}

func TestTuneNoSelectorGlyph(t *testing.T) {
	tn := NewTune("app/launch/tune", "high", true)
	view := plain(tn.View())
	if strings.Contains(view, "▸") {
		t.Errorf("tune view uses a cursor glyph, want cursorless highlight-only:\n%s", view)
	}
}
