package frame

import (
	"testing"

	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func TestLargeMarkReducedMotion(t *testing.T) {
	for _, env := range []struct{ k, v string }{
		{"NO_ANIMATE", "1"},
		{"NO_COLOR", "1"},
	} {
		t.Run(env.k, func(t *testing.T) {
			t.Setenv(env.k, env.v)
			m := New("mirabilis", "v0.0.0", nil)
			got := m.largeMark()
			if got != uistr.LogoLargeStatic {
				t.Errorf("largeMark() under %s=%s = %q, want LogoLargeStatic (%q)", env.k, env.v, got, uistr.LogoLargeStatic)
			}
		})
	}
}

func TestLogoLargeRotatesNFrames(t *testing.T) {
	if len(logoLargeFrames) < 8 {
		t.Errorf("logoLargeFrames has %d frames, want >= 8 (INV §1 discrete spinner)", len(logoLargeFrames))
	}
	m := New("mirabilis", "v0.0.0", nil)
	seen := make(map[string]bool)
	for i := range logoLargeFrames {
		m.chromeFrame = i
		seen[m.largeMark()] = true
	}
	if len(seen) < 2 {
		t.Error("largeMark() returns same frame for all chromeFrame values — not rotating (INV §1)")
	}
}
