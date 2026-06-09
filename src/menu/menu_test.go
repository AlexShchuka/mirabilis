package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func sizedMenu(st Status) menuModel {
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
		name string
		give Status
		want map[string]bool
	}{
		{
			name: "container down disables container actions",
			give: Status{ContainerUp: false},
			want: map[string]bool{"launch": false, "plugins": true, "harness": true, "stacks": false, "vscode": true, "quit": false},
		},
		{
			name: "container up enables everything",
			give: Status{ContainerUp: true},
			want: map[string]bool{"launch": false, "plugins": false, "harness": false, "stacks": false, "vscode": false, "quit": false},
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
	want := []string{"launch", "plugins", "harness", "stacks", "vscode", "quit"}
	for i, action := range want {
		m := sizedMenu(Status{ContainerUp: true})
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
	m := sizedMenu(Status{ContainerUp: false})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if it, _ := m.selected(); it.action != "stacks" {
		t.Errorf("down from launch landed on %q, want stacks", it.action)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if it, _ := m.selected(); it.action != "quit" {
		t.Errorf("down from stacks landed on %q, want quit", it.action)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if it, _ := m.selected(); it.action != "stacks" {
		t.Errorf("up from quit landed on %q, want stacks", it.action)
	}
}

func TestMenuEnterOnDisabledDoesNothing(t *testing.T) {
	m := sizedMenu(Status{ContainerUp: false})
	m.list.Select(1)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if action, ok := menuChoice(cmd); ok {
		t.Errorf("enter on disabled item dispatched %q, want nothing", action)
	}
}

func TestMenuQuitKeys(t *testing.T) {
	keys := []tea.KeyPressMsg{{Code: tea.KeyEscape}, {Code: 'q', Text: "q"}}
	for _, key := range keys {
		m := sizedMenu(Status{ContainerUp: true})
		_, cmd := m.Update(key)
		if action, _ := menuChoice(cmd); action != "quit" {
			t.Errorf("key %q dispatched %q, want quit", key.String(), action)
		}
	}
}
