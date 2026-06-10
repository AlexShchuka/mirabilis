package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/ui"
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

func TestMenuItemMethods(t *testing.T) {
	it := item{action: "act", title: "MyTitle", desc: "MyDesc", disabled: false}
	if it.FilterValue() != "MyTitle" {
		t.Errorf("FilterValue = %q, want MyTitle", it.FilterValue())
	}
	if it.Title() != "MyTitle" {
		t.Errorf("Title = %q, want MyTitle", it.Title())
	}
	if it.Description() != "MyDesc" {
		t.Errorf("Description = %q, want MyDesc", it.Description())
	}
}

func TestMenuHeader(t *testing.T) {
	tests := []struct {
		name    string
		st      provision.Status
		wantIn  []string
		wantOut []string
	}{
		{
			name:   "stale workspace",
			st:     provision.Status{Stale: true},
			wantIn: []string{ui.MenuStale},
		},
		{
			name:   "commits behind",
			st:     provision.Status{CommitsBehind: 3},
			wantIn: []string{"3 behind origin/main"},
		},
		{
			name:   "harness off",
			st:     provision.Status{Harness: "off"},
			wantIn: []string{ui.MenuHarnessOff},
		},
		{
			name:   "harness missing",
			st:     provision.Status{Harness: "missing"},
			wantIn: []string{ui.MenuHarnessMissing},
		},
		{
			name:   "harness unknown",
			st:     provision.Status{Harness: "unknown"},
			wantIn: []string{ui.MenuHarnessUnknown},
		},
		{
			name:    "no issues",
			st:      provision.Status{},
			wantOut: []string{ui.MenuStale, ui.MenuHarnessOff},
		},
		{
			name:   "provision warned",
			st:     provision.Status{ProvisionWarn: "3/12 warned"},
			wantIn: []string{"provision: 3/12 warned"},
		},
		{
			name:    "provision ok (empty string)",
			st:      provision.Status{ProvisionWarn: ""},
			wantOut: []string{"provision:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := header(tt.st)
			for _, s := range tt.wantIn {
				if !strings.Contains(h, s) {
					t.Errorf("header(%+v) = %q, want to contain %q", tt.st, h, s)
				}
			}
			for _, s := range tt.wantOut {
				if strings.Contains(h, s) {
					t.Errorf("header(%+v) = %q, should not contain %q", tt.st, h, s)
				}
			}
		})
	}
}

func TestMenuInit_ReturnsNil(t *testing.T) {
	m := newMenu(provision.Status{})
	if cmd := m.Init(); cmd != nil {
		t.Error("menuModel.Init() should return nil")
	}
}

func TestMenuView_ContainsHint(t *testing.T) {
	m := sizedMenu(provision.Status{ContainerUp: true})
	v := m.View()
	if !strings.Contains(v, ui.MenuHint) {
		t.Errorf("menu.View() missing hint %q, got:\n%s", ui.MenuHint, v)
	}
}

func TestMenuView_ContainsItems(t *testing.T) {
	m := sizedMenu(provision.Status{ContainerUp: true})
	v := m.View()
	for _, title := range []string{
		ui.MenuActionLaunch,
		ui.MenuActionReset,
		ui.MenuActionQuit,
	} {
		if !strings.Contains(v, title) {
			t.Errorf("menu.View() missing item %q, got:\n%s", title, v)
		}
	}
}

func TestMenuContainerOffShowsDesc(t *testing.T) {
	m := sizedMenu(provision.Status{ContainerUp: false})
	v := m.View()
	if !strings.Contains(v, ui.MenuDescContainerOff) {
		t.Errorf("menu.View() missing disabled desc %q, got:\n%s", ui.MenuDescContainerOff, v)
	}
}
