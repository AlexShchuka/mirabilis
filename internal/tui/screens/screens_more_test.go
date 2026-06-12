package screens

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
)

func TestMenuNotice(t *testing.T) {
	m := NewMenu("app/menu").WithNotice("test notice")
	if m.Notice() != "test notice" {
		t.Errorf("Notice() = %q, want %q", m.Notice(), "test notice")
	}
}

func TestMenuNoticeEmpty(t *testing.T) {
	m := NewMenu("app/menu")
	if m.Notice() != "" {
		t.Errorf("Notice() = %q, want empty", m.Notice())
	}
}

func TestHarnessInit(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	cmd := h.Init()
	if cmd == nil {
		t.Error("Harness.Init() = nil, want non-nil (form init)")
	}
}

func TestHarnessDoneUpdateIgnored(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	h.done = true
	scr, cmd := h.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("Update on done harness emitted cmd, want nil")
	}
	if _, ok := scr.(Harness); !ok {
		t.Errorf("Update on done harness returned %T, want Harness", scr)
	}
}

func TestHarnessDoneViewEmpty(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	h.done = true
	if h.View() != "" {
		t.Errorf("View() on done harness = %q, want empty", h.View())
	}
}

func TestHarnessFormUpdatePassthrough(t *testing.T) {
	h := NewHarness("app/harness", HarnessOn, "")
	scr, _ := h.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if _, ok := scr.(Harness); !ok {
		t.Errorf("Update returned %T, want Harness", scr)
	}
}

func TestHarnessSubmitEmitsScreenResult(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	result := h.form.SubmitCmd()
	if result == nil {
		t.Fatal("SubmitCmd() = nil")
	}
}

func TestHarnessCancelEmitsScreenPop(t *testing.T) {
	h := NewHarness("app/harness", HarnessOff, "")
	msg := h.form.CancelCmd()
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Errorf("CancelCmd() = %T, want bus.ScreenPop", msg)
	}
}

func TestHarnessLabelOn(t *testing.T) {
	label := harnessLabel(HarnessOn)
	if label == "" {
		t.Error("harnessLabel(on) is empty")
	}
}

func TestResetInit(t *testing.T) {
	r := NewReset("app/reset")
	cmd := r.Init()
	if cmd == nil {
		t.Error("Reset.Init() = nil, want non-nil (form init)")
	}
}

func TestResetDoneUpdateIgnored(t *testing.T) {
	r := NewReset("app/reset")
	r.done = true
	scr, cmd := r.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("Update on done reset emitted cmd, want nil")
	}
	if _, ok := scr.(Reset); !ok {
		t.Errorf("Update on done reset returned %T, want Reset", scr)
	}
}

func TestResetDoneViewEmpty(t *testing.T) {
	r := NewReset("app/reset")
	r.done = true
	if r.View() != "" {
		t.Errorf("View() on done reset = %q, want empty", r.View())
	}
}

func TestResetFormUpdatePassthrough(t *testing.T) {
	r := NewReset("app/reset")
	scr, _ := r.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if _, ok := scr.(Reset); !ok {
		t.Errorf("Update returned %T, want Reset", scr)
	}
}

func TestResetSubmitConfirmEmitsScreenResult(t *testing.T) {
	r := NewReset("app/reset")
	*r.val = true
	msg := r.form.SubmitCmd()
	if _, ok := msg.(bus.ScreenResult); !ok {
		t.Errorf("SubmitCmd with val=true = %T, want bus.ScreenResult", msg)
	}
}

func TestResetSubmitCancelEmitsScreenPop(t *testing.T) {
	r := NewReset("app/reset")
	*r.val = false
	msg := r.form.SubmitCmd()
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Errorf("SubmitCmd with val=false = %T, want bus.ScreenPop", msg)
	}
}

func TestResetFormCancelEmitsScreenPop(t *testing.T) {
	r := NewReset("app/reset")
	msg := r.form.CancelCmd()
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Errorf("CancelCmd = %T, want bus.ScreenPop", msg)
	}
}

func TestTelegramInit(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	cmd := s.Init()
	if cmd == nil {
		t.Error("Telegram.Init() = nil, want non-nil (form init)")
	}
}

func TestTelegramDoneUpdateIgnored(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	s.done = true
	scr, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("Update on done telegram emitted cmd, want nil")
	}
	if _, ok := scr.(Telegram); !ok {
		t.Errorf("Update on done telegram returned %T, want Telegram", scr)
	}
}

func TestTelegramDoneViewEmpty(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	s.done = true
	if s.View() != "" {
		t.Errorf("View() on done telegram = %q, want empty", s.View())
	}
}

func TestTelegramFormUpdatePassthrough(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	scr, _ := s.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if _, ok := scr.(Telegram); !ok {
		t.Errorf("Update returned %T, want Telegram", scr)
	}
}

func TestTelegramSubmitTokenEmitsScreenResult(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	*s.tok = "my-token"
	*s.sel = "Configure"
	msg := s.form.SubmitCmd()
	if result, ok := msg.(bus.ScreenResult); !ok {
		t.Errorf("SubmitCmd with token = %T, want bus.ScreenResult", msg)
	} else if result.Value != "my-token" {
		t.Errorf("SubmitCmd value = %v, want my-token", result.Value)
	}
}

func TestTelegramSubmitSkipEmitsScreenResultWithSkip(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	*s.sel = TelegramSkip
	msg := s.form.SubmitCmd()
	if result, ok := msg.(bus.ScreenResult); !ok {
		t.Errorf("SubmitCmd on skip = %T, want bus.ScreenResult", msg)
	} else if result.Value != TelegramSkip {
		t.Errorf("SubmitCmd skip value = %v, want TelegramSkip", result.Value)
	}
}

func TestTelegramSubmitEmptyTokenEmitsSkip(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	*s.tok = ""
	*s.sel = "Configure"
	msg := s.form.SubmitCmd()
	if result, ok := msg.(bus.ScreenResult); !ok {
		t.Errorf("SubmitCmd with empty token = %T, want bus.ScreenResult", msg)
	} else if result.Value != TelegramSkip {
		t.Errorf("SubmitCmd empty-token value = %v, want TelegramSkip", result.Value)
	}
}

func TestTelegramFormCancelEmitsScreenPop(t *testing.T) {
	s := NewTelegram("app/telegram", false)
	msg := s.form.CancelCmd()
	if _, ok := msg.(bus.ScreenPop); !ok {
		t.Errorf("CancelCmd = %T, want bus.ScreenPop", msg)
	}
}
