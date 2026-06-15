package screens

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestMenuID(t *testing.T) {
	m := NewMenu("app/menu")
	if m.ID() != "app/menu" {
		t.Errorf("ID() = %q, want app/menu", m.ID())
	}
	if m.Init() != nil {
		t.Error("Init() should be nil")
	}
}

func TestMenuQuitKeys(t *testing.T) {
	m := NewMenu("app/menu")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Update(esc) returned nil cmd")
	}
	got, ok := cmd().(bus.MenuChosen)
	if !ok || got.Action != ActionQuit {
		t.Errorf("Update(esc) emitted %v, want MenuChosen{quit}", got)
	}

	m = NewMenu("app/menu")
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil {
		t.Errorf("Update(q) emitted %v, want nil (q consumed at app depth 1)", cmd())
	}
}

func TestMenuEnvelopeUnwrap(t *testing.T) {
	m := NewMenu("app/menu")
	_, cmd := m.Update(bus.Envelope{To: "app/menu", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	if cmd == nil {
		t.Fatal("enveloped key returned nil cmd")
	}
	if got, ok := cmd().(bus.MenuChosen); !ok || got.Action != ActionQuit {
		t.Errorf("got %v, want MenuChosen{quit}", got)
	}
}

func TestMenuOtherKeysIgnored(t *testing.T) {
	m := NewMenu("app/menu")
	scr, cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd != nil {
		t.Error("j should not emit a cmd from the menu screen")
	}
	if _, ok := scr.(Menu); !ok {
		t.Errorf("Update returned %T, want Menu", scr)
	}
}

func TestMenuViewWelcomeAndNotice(t *testing.T) {
	view := plain(NewMenu("app/menu").View())
	for _, want := range []string{uistr.AppName, uistr.WelcomeHint} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Launch") || strings.Contains(view, "setup pipeline") {
		t.Errorf("menu screen still renders the item list (moved to frame):\n%s", view)
	}
	if strings.Contains(view, "launch canceled") {
		t.Error("View() shows a notice without WithNotice")
	}

	view = plain(NewMenu("app/menu").WithNotice("launch canceled").View())
	if !strings.Contains(view, "launch canceled") {
		t.Errorf("View() missing notice:\n%s", view)
	}
}

func TestLargeMarkReducedMotion(t *testing.T) {
	for _, env := range []struct{ k, v string }{
		{"NO_ANIMATE", "1"},
		{"NO_COLOR", "1"},
	} {
		t.Run(env.k, func(t *testing.T) {
			t.Setenv(env.k, env.v)
			m := NewMenu("app/menu")
			got := m.largeMark()
			if got != uistr.LogoLargeStatic {
				t.Errorf("largeMark() under %s=%s = %q, want LogoLargeStatic (%q)", env.k, env.v, got, uistr.LogoLargeStatic)
			}
		})
	}
}

func TestLogoLargeFramesNoSolidDot(t *testing.T) {
	frames := []string{
		uistr.LogoLargeFrameA,
		uistr.LogoLargeFrameB,
		uistr.LogoLargeFrameC,
		uistr.LogoLargeFrameD,
		uistr.LogoLargeFrameE,
		uistr.LogoLargeFrameF,
		uistr.LogoLargeFrameG,
		uistr.LogoLargeFrameH,
		uistr.LogoLargeStatic,
	}
	for i, f := range frames {
		if strings.Contains(f, "⊙") {
			t.Errorf("frame %d contains ⊙ (center-dot) — must be ○ (INV §1)", i)
		}
		if !strings.Contains(f, "○") {
			t.Errorf("frame %d does not contain ○ (ring) (INV §1)", i)
		}
	}
}

func TestLogoLargeRotatesNFrames(t *testing.T) {
	if len(logoLargeFrames) < 8 {
		t.Errorf("logoLargeFrames has %d frames, want >= 8 (INV §1 discrete spinner)", len(logoLargeFrames))
	}
	m := NewMenu("app/menu")
	seen := make(map[string]bool)
	for i := range logoLargeFrames {
		m.chromeFrame = i
		seen[m.largeMark()] = true
	}
	if len(seen) < 2 {
		t.Error("largeMark() returns same frame for all chromeFrame values — not rotating (INV §1)")
	}
}

func TestMenuItemsActions(t *testing.T) {
	want := []string{ActionLaunch, ActionHarness, ActionVSCode, ActionReset, ActionQuit}
	items := MenuItems()
	if len(items) != len(want) {
		t.Fatalf("MenuItems() len = %d, want %d", len(items), len(want))
	}
	for i, it := range items {
		if it.Action != want[i] {
			t.Errorf("item %d action = %q, want %q", i, it.Action, want[i])
		}
		if !it.Enabled {
			t.Errorf("item %q enabled = false, want true", it.Action)
		}
		if it.Title == "" {
			t.Errorf("item %q has no title", it.Action)
		}
	}
}
