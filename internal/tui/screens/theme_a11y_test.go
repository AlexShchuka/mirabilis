package screens

import (
	"strings"
	"testing"

	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func TestResetCarriesDangerGlyphUnderNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	r := NewReset("app/reset")
	r.Init()
	view := plain(r.View())
	if !strings.Contains(view, uistr.GlyphDanger) {
		t.Errorf("reset view lacks the danger glyph under NO_COLOR; danger must survive without color (WCAG 1.4.1):\n%s", view)
	}
	if !strings.Contains(view, uistr.FormConfirmReset) {
		t.Errorf("reset view lacks the explicit destructive affirmative copy:\n%s", view)
	}
}

func TestFormsConstructUnderAccessible(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")
	if plain(NewReset("app/reset").View()) == "" {
		t.Error("reset form view empty under ACCESSIBLE=1")
	}
	if plain(NewHarness("app/harness", HarnessOff, "").View()) == "" {
		t.Error("harness form view empty under ACCESSIBLE=1")
	}
	if plain(NewTelegram("app/telegram", false).View()) == "" {
		t.Error("telegram form view empty under ACCESSIBLE=1")
	}
}
