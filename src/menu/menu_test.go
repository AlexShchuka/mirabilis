package main

import (
	"encoding/json"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func driveTo(t *testing.T, m Model, index int) Model {
	t.Helper()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = next.(Model)
	for i := 0; i < index; i++ {
		next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = next.(Model)
	}
	return m
}

func TestEnterDispatchesSelectedAction(t *testing.T) {
	want := []string{"launch", "update", "plugins", "harness", "stacks", "vscode", "secrets", "theme", "quit"}
	for i, action := range want {
		m := driveTo(t, New(Status{}), i)
		final, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if got := Action(final); got != action {
			t.Errorf("row %d: enter dispatched %q, want %q", i, got, action)
		}
	}
}

func TestQuitKeys(t *testing.T) {
	keys := []tea.KeyPressMsg{
		{Code: 'c', Mod: tea.ModCtrl},
		{Code: tea.KeyEscape},
		{Code: 'q', Text: "q"},
	}
	for _, key := range keys {
		final, _ := New(Status{}).Update(key)
		if got := Action(final); got != "quit" {
			t.Errorf("key %q dispatched %q, want quit", key.String(), got)
		}
	}
}

func TestItemActionsMatchDispatcher(t *testing.T) {
	dispatcher := map[string]bool{
		"launch": true, "update": true, "plugins": true, "harness": true,
		"stacks": true, "vscode": true, "secrets": true, "theme": true, "quit": true,
	}
	m := New(Status{})
	seen := map[string]bool{}
	for i, li := range m.list.Items() {
		it, ok := li.(item)
		if !ok {
			t.Fatalf("row %d is not a menu item", i)
		}
		if !dispatcher[it.action] {
			t.Errorf("row %d action %q is not handled by the main.go dispatcher", i, it.action)
		}
		seen[it.action] = true
	}
	for action := range dispatcher {
		if !seen[action] {
			t.Errorf("dispatcher action %q has no menu item", action)
		}
	}
}

func TestStatusUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  Status
	}{
		{"valid", `{"commitsBehind":3,"stale":true,"harness":"missing"}`, Status{CommitsBehind: 3, Stale: true, Harness: "missing"}},
		{"empty object", `{}`, Status{}},
		{"unknown harness", `{"harness":"unknown"}`, Status{Harness: "unknown"}},
		{"partial", `{"stale":true}`, Status{Stale: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s Status
			if err := json.Unmarshal([]byte(c.input), &s); err != nil {
				t.Fatalf("unmarshal %q: %v", c.input, err)
			}
			if s != c.want {
				t.Errorf("unmarshal %q = %+v, want %+v", c.input, s, c.want)
			}
		})
	}
}

func TestStatusUnmarshalGarbage(t *testing.T) {
	for _, input := range []string{"", "not json", "[1,2,3]", "null"} {
		var s Status
		_ = json.Unmarshal([]byte(input), &s)
		if s != (Status{}) {
			t.Errorf("garbage %q left non-zero status %+v", input, s)
		}
	}
}
