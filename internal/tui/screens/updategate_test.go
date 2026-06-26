package screens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func TestUpdateGateID(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	if g.ID() != "app/launch/gate" {
		t.Errorf("ID() = %q", g.ID())
	}
	if g.Init() != nil {
		t.Error("Init() should be nil")
	}
}

func TestUpdateGateOffersAllFourChoices(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	keys := make([]string, len(g.options))
	for i, o := range g.options {
		keys[i] = o.key
	}
	want := []string{GateSkip, GateSelf, GatePacks, GateAll}
	if len(keys) != len(want) {
		t.Fatalf("options = %v, want %v", keys, want)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("option %d = %q, want %q", i, keys[i], k)
		}
	}
}

func TestUpdateGateDefaultCursorIsSkip(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "v9.9.9")
	if g.cursor != 0 {
		t.Fatalf("default cursor = %d, want 0 (Skip)", g.cursor)
	}
	if g.options[g.cursor].key != GateSkip {
		t.Errorf("default option = %q, want %q", g.options[g.cursor].key, GateSkip)
	}
}

func TestUpdateGateEnterEmitsSelectedChoice(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	scr, _ := g.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	g = scr.(UpdateGate)
	_, cmd := g.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := emit(cmd)
	res, ok := msg.(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", msg)
	}
	if res.Value != GateSelf {
		t.Errorf("ScreenResult.Value = %v, want %q", res.Value, GateSelf)
	}
}

func TestUpdateGateEnterOnDefaultEmitsSkip(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	_, cmd := g.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	res, ok := emit(cmd).(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", emit(cmd))
	}
	if res.Value != GateSkip {
		t.Errorf("default enter Value = %v, want %q", res.Value, GateSkip)
	}
}

func TestUpdateGateEscEmitsSkip(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	scr, _ := g.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	g = scr.(UpdateGate)
	_, cmd := g.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	res, ok := emit(cmd).(bus.ScreenResult)
	if !ok {
		t.Fatalf("esc: got %T, want bus.ScreenResult", emit(cmd))
	}
	if res.Value != GateSkip {
		t.Errorf("esc Value = %v, want %q (esc skips the gate)", res.Value, GateSkip)
	}
}

func TestUpdateGateNavigationClamps(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	scr, _ := g.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if scr.(UpdateGate).cursor != 0 {
		t.Errorf("up at top: cursor = %d, want 0 (clamped)", scr.(UpdateGate).cursor)
	}
	g = scr.(UpdateGate)
	for range g.options {
		scr, _ = g.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		g = scr.(UpdateGate)
	}
	if g.cursor != len(g.options)-1 {
		t.Errorf("down past end: cursor = %d, want %d (clamped)", g.cursor, len(g.options)-1)
	}
}

func TestUpdateGateEnvelopeUnwrap(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	_, cmd := g.Update(bus.Envelope{To: "app/launch/gate", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	if _, ok := emit(cmd).(bus.ScreenResult); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenResult", emit(cmd))
	}
}

func TestUpdateGateRendersUpToDateFreshness(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "")
	view := plain(g.View())
	if !strings.Contains(view, uistr.GateTitle) {
		t.Errorf("view missing title: %q", view)
	}
	if !strings.Contains(view, "abc1234") {
		t.Errorf("view missing current version: %q", view)
	}
	if !strings.Contains(view, uistr.GateUpToDate) {
		t.Errorf("view missing up-to-date marker: %q", view)
	}
	for _, label := range []string{uistr.GateOptionSkip, uistr.GateOptionSelf, uistr.GateOptionPacks, uistr.GateOptionAll} {
		if !strings.Contains(view, label) {
			t.Errorf("view missing option %q: %q", label, view)
		}
	}
}

func TestUpdateGateRendersOutdatedFreshness(t *testing.T) {
	g := NewUpdateGate("app/launch/gate", "abc1234", "v9.9.9")
	view := plain(g.View())
	if strings.Contains(view, uistr.GateUpToDate) {
		t.Errorf("outdated gate shows up-to-date marker: %q", view)
	}
	if !strings.Contains(view, "v9.9.9") {
		t.Errorf("view missing latest tag: %q", view)
	}
	if !strings.Contains(view, uistr.GateOutdatedSuffix) {
		t.Errorf("view missing 'available' suffix: %q", view)
	}
}
