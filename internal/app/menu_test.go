package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/provision"
)

func sizedMenu(st provision.Status) menuModel {
	m := newMenu(st)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	return next
}

func menuChoice(cmd tea.Cmd) (string, bool) {
	if cmd == nil {
		return "", false
	}
	c, ok := cmd().(menuChoiceMsg)
	if !ok {
		return "", false
	}
	return c.action, true
}

func TestMenuItemsDisabled(t *testing.T) {
	tests := []struct {
		want map[string]bool
		name string
		give provision.Status
	}{
		{
			name: "container down disables container actions",
			give: provision.Status{ContainerUp: false},
			want: map[string]bool{"launch": false, "harness": true, "vscode": true, "reset": false, "quit": false},
		},
		{
			name: "container up enables everything",
			give: provision.Status{ContainerUp: true},
			want: map[string]bool{"launch": false, "harness": false, "vscode": false, "reset": false, "quit": false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, li := range menuItems(tt.give) {
				it, ok := li.(item)
				if !ok {
					t.Fatal("menu item is not of type item")
				}
				if it.disabled != tt.want[it.action] {
					t.Errorf("%q disabled=%v, want %v", it.action, it.disabled, tt.want[it.action])
				}
			}
		})
	}
}

func TestMenuEnterDispatchesEnabledAction(t *testing.T) {
	want := []string{"launch", "harness", "vscode", "reset", "quit"}
	for i, action := range want {
		m := sizedMenu(provision.Status{ContainerUp: true})
		for j := 0; j < i; j++ {
			m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		got, ok := menuChoice(cmd)
		if !ok || got != action {
			t.Errorf("row %d: dispatched %q (ok=%v), want %q", i, got, ok, action)
		}
	}
}

func TestMenuNavigationSkipsDisabled(t *testing.T) {
	m := sizedMenu(provision.Status{ContainerUp: false})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if it, _ := m.selected(); it.action != "reset" {
		t.Errorf("down from launch landed on %q, want reset", it.action)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if it, _ := m.selected(); it.action != "launch" {
		t.Errorf("up from reset landed on %q, want launch", it.action)
	}
}

func TestMenuEnterOnDisabledDoesNothing(t *testing.T) {
	m := sizedMenu(provision.Status{ContainerUp: false})
	m.list.Select(1)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if action, ok := menuChoice(cmd); ok {
		t.Errorf("enter on disabled item dispatched %q, want nothing", action)
	}
}

func TestMenuQuitKeys(t *testing.T) {
	keys := []tea.KeyPressMsg{{Code: tea.KeyEscape}, {Code: 'q', Text: "q"}}
	for _, key := range keys {
		m := sizedMenu(provision.Status{ContainerUp: true})
		_, cmd := m.Update(key)
		if action, _ := menuChoice(cmd); action != "quit" {
			t.Errorf("key %q dispatched %q, want quit", key.String(), action)
		}
	}
}
