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

func TestTelegramID(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	if s.ID() != "app/telegram" {
		t.Fatalf("ID() = %q", s.ID())
	}
}

func TestTelegramEscEmitsScreenPop(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	_, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestTelegramEnvelopeUnwrapEsc(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	_, cmd := s.Update(bus.Envelope{To: "app/telegram", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestTelegramTokenNotEchoedInView(t *testing.T) {
	const secret = "1234567890:secret_token_value"
	s := NewTelegram("app/telegram", false)
	s2, _ := s.Update(tea.KeyPressMsg{Code: 0, Text: secret})
	view := plain(s2.View())
	if strings.Contains(view, secret) {
		t.Errorf("token echoed in plaintext in View(): %q", view)
	}
	if strings.Contains(view, "secret_token_value") {
		t.Errorf("token substring echoed in View(): %q", view)
	}
}

func TestTelegramViewNotEmpty(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	if plain(s.View()) == "" {
		t.Error("View() is empty before any interaction")
	}
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

func TestResetID(t *testing.T) {
	r := NewReset("app/reset")
	if r.ID() != "app/reset" {
		t.Fatalf("ID() = %q", r.ID())
	}
}

func TestResetEscEmitsScreenPop(t *testing.T) {
	r := NewReset("app/reset")
	_, cmd := r.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestResetEnvelopeUnwrap(t *testing.T) {
	r := NewReset("app/reset")
	_, cmd := r.Update(bus.Envelope{To: "app/reset", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestResetViewNotEmpty(t *testing.T) {
	r := NewReset("app/reset")
	if plain(r.View()) == "" {
		t.Error("View() is empty")
	}
}

func TestHarnessID(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	if h.ID() != "app/harness" {
		t.Fatalf("ID() = %q", h.ID())
	}
}

func TestHarnessEscEmitsScreenPop(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	_, cmd := h.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestHarnessEnvelopeUnwrap(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	_, cmd := h.Update(bus.Envelope{To: "app/harness", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	msg := emit(cmd)
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenPop", msg)
	}
}

func TestHarnessViewNotEmpty(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	if plain(h.View()) == "" {
		t.Error("View() is empty")
	}
}

func TestHarnessPrefillsLastChoice(t *testing.T) {
	tests := []struct {
		last string
		want string
	}{
		{HarnessReinstall, HarnessReinstall},
		{HarnessOn, HarnessOn},
		{HarnessOff, HarnessOff},
		{"", HarnessOn},
		{"garbage", HarnessOn},
	}
	for _, tt := range tests {
		h := NewHarness("app/harness", HarnessOff, tt.last)
		if *h.val != tt.want {
			t.Errorf("NewHarness(last=%q): default value = %q, want %q", tt.last, *h.val, tt.want)
		}
	}
}

func TestTelegramSmartDefaultSkipWhenConfigured(t *testing.T) {
	configured := NewTelegram("app/telegram", true)
	if *configured.sel != TelegramSkip {
		t.Errorf("configured telegram default sel = %q, want %q", *configured.sel, TelegramSkip)
	}

	fresh := NewTelegram("app/telegram", false)
	if *fresh.sel == TelegramSkip {
		t.Error("fresh telegram pre-selected Skip, want unset so Configure is reachable")
	}
}

func TestHarnessConsts(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"HarnessOn", HarnessOn},
		{"HarnessOff", HarnessOff},
		{"HarnessReinstall", HarnessReinstall},
		{"TelegramSkip", TelegramSkip},
	}
	for _, c := range cases {
		if c.val == "" {
			t.Errorf("const %s is empty", c.name)
		}
	}
}

func TestHarnessLabelForCurrentStates(t *testing.T) {
	cases := []struct {
		input string
	}{
		{HarnessOff},
		{"missing"},
		{"unknown"},
		{"on"},
	}
	for _, c := range cases {
		label := harnessLabel(c.input)
		if label == "" {
			t.Errorf("harnessLabel(%q) is empty", c.input)
		}
	}
}
