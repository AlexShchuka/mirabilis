package frame

import (
	"testing"

	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func TestSmallGlyphReducedMotion(t *testing.T) {
	for _, env := range []struct{ k, v string }{
		{"NO_ANIMATE", "1"},
		{"NO_COLOR", "1"},
	} {
		t.Run(env.k, func(t *testing.T) {
			t.Setenv(env.k, env.v)
			m := New("mirabilis", "v0.0.0", nil)
			got := m.smallGlyph()
			frames := []rune(uistr.LogoSmallFrames)
			want := string(frames[0])
			if got != want {
				t.Errorf("smallGlyph() under %s=%s = %q, want %q", env.k, env.v, got, want)
			}
		})
	}
}
