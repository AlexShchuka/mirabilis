package screens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
)

func emit(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestGHAuthID(t *testing.T) {
	g := NewGHAuth("app/ghauth", "ABCD-1234", "https://github.com/login/device")
	if g.ID() != "app/ghauth" {
		t.Fatalf("ID() = %q", g.ID())
	}
}

func TestGHAuthInit(t *testing.T) {
	g := NewGHAuth("app/ghauth", "ABCD-1234", "https://github.com/login/device")
	if g.Init() != nil {
		t.Error("Init() should be nil")
	}
}

func TestGHAuthRendersCodeAndURL(t *testing.T) {
	g := NewGHAuth("app/ghauth", "ABCD-1234", "https://github.com/login/device")
	view := plain(g.View())
	if !strings.Contains(view, "ABCD-1234") {
		t.Errorf("code not in view: %q", view)
	}
	if !strings.Contains(view, "https://github.com/login/device") {
		t.Errorf("url not in view: %q", view)
	}
}

func TestGHAuthAppendsStepLine(t *testing.T) {
	g := NewGHAuth("app/ghauth", "ABCD-1234", "https://github.com/login/device")
	g2, _ := g.Update(bus.StepEvent{Kind: bus.StepLine, Line: "device code confirmed"})
	view := plain(g2.View())
	if !strings.Contains(view, "device code confirmed") {
		t.Errorf("step line not in view: %q", view)
	}
}

func TestGHAuthIgnoresNonLinekEvents(t *testing.T) {
	g := NewGHAuth("app/ghauth", "ABCD-1234", "https://github.com/login/device")
	before := len(g.lines)
	g2, _ := g.Update(bus.StepEvent{Kind: bus.StepDone})
	if scr, ok := g2.(GHAuth); ok && len(scr.lines) != before {
		t.Errorf("non-line event changed lines count: %d → %d", before, len(scr.lines))
	}
}

func TestGHAuthEscEmitsScreenPop(t *testing.T) {
	g := NewGHAuth("app/ghauth", "ABCD-1234", "https://github.com/login/device")
	_, cmd := g.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestGHAuthEnvelopeUnwrap(t *testing.T) {
	g := NewGHAuth("app/ghauth", "ABCD-1234", "https://github.com/login/device")
	_, cmd := g.Update(bus.Envelope{To: "app/ghauth", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestGHAuthCKeyEmitsCopyRequest(t *testing.T) {
	const code = "ABCD-1234"
	g := NewGHAuth("app/ghauth", code, "https://github.com/login/device")
	_, cmd := g.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	msg := emit(cmd)
	cr, ok := msg.(bus.CopyRequest)
	if !ok {
		t.Fatalf("c key: got %T, want bus.CopyRequest", msg)
	}
	if cr.Text != code {
		t.Errorf("CopyRequest.Text = %q, want %q", cr.Text, code)
	}
}
