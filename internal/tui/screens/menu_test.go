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

func TestMenuItemsActions(t *testing.T) {
	want := []string{ActionLaunch, ActionHarness, ActionTelegram, ActionVSCode, ActionReset, ActionQuit}
	items := MenuItems()
	if len(items) != len(want) {
		t.Fatalf("MenuItems() len = %d, want %d", len(items), len(want))
	}
	for i, it := range items {
		if it.Action != want[i] {
			t.Errorf("item %d action = %q, want %q", i, it.Action, want[i])
		}
		if !it.Enabled {
			t.Errorf("item %q disabled by default", it.Action)
		}
		if it.Title == "" {
			t.Errorf("item %q has no title", it.Action)
		}
	}
}
