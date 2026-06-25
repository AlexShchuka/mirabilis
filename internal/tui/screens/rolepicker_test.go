package screens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

func roleOpts() []RoleOption {
	return []RoleOption{
		{Key: "grind", Effort: "xhigh", Harness: false},
		{Key: "raid", Effort: "max", Harness: true, Default: true},
		{Key: "pvp", Effort: "max", Harness: false},
	}
}

func TestRolePickerID(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	if r.ID() != "app/launch/role" {
		t.Errorf("ID() = %q", r.ID())
	}
	if r.Init() != nil {
		t.Error("Init() should be nil")
	}
}

func TestRolePickerCursorStartsOnDefault(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	if r.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (raid is default)", r.cursor)
	}
}

func TestRolePickerCursorStartsAtZeroWithoutDefault(t *testing.T) {
	r := NewRolePicker("app/launch/role", []RoleOption{{Key: "grind"}, {Key: "raid"}})
	if r.cursor != 0 {
		t.Errorf("cursor = %d, want 0", r.cursor)
	}
}

func TestRolePickerNavigation(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())

	scr, _ := r.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	r = scr.(RolePicker)
	if r.cursor != 0 {
		t.Errorf("after up: cursor = %d, want 0", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	r = scr.(RolePicker)
	if r.cursor != 0 {
		t.Errorf("up at top: cursor = %d, want 0 (clamped)", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	r = scr.(RolePicker)
	scr, _ = r.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	r = scr.(RolePicker)
	if r.cursor != 2 {
		t.Errorf("after j j: cursor = %d, want 2", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	r = scr.(RolePicker)
	if r.cursor != 2 {
		t.Errorf("down at bottom: cursor = %d, want 2 (clamped)", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	r = scr.(RolePicker)
	if r.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", r.cursor)
	}
}

func TestRolePickerEnterEmitsResultWithSelectedKey(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	scr, _ := r.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	r = scr.(RolePicker)
	_, cmd := r.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := emit(cmd)
	res, ok := msg.(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", msg)
	}
	if res.Value != "grind" {
		t.Errorf("ScreenResult.Value = %v, want grind", res.Value)
	}
}

func TestRolePickerEnterOnDefaultEmitsDefaultKey(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	_, cmd := r.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := emit(cmd)
	res, ok := msg.(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", msg)
	}
	if res.Value != "raid" {
		t.Errorf("ScreenResult.Value = %v, want raid", res.Value)
	}
}

func TestRolePickerEscEmitsScreenPop(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	_, cmd := r.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := emit(cmd).(bus.ScreenPop); !ok {
		t.Fatalf("esc: got %T, want bus.ScreenPop", emit(cmd))
	}
}

func TestRolePickerEnvelopeUnwrap(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	_, cmd := r.Update(bus.Envelope{To: "app/launch/role", Msg: tea.KeyPressMsg{Code: tea.KeyEscape}})
	if _, ok := emit(cmd).(bus.ScreenPop); !ok {
		t.Fatalf("envelope esc: got %T, want bus.ScreenPop", emit(cmd))
	}
}

func TestRolePickerRendersAllOptions(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	view := plain(r.View())
	for _, key := range []string{"grind", "raid", "pvp"} {
		if !strings.Contains(view, key) {
			t.Errorf("view missing role %q: %q", key, view)
		}
	}
	if !strings.Contains(view, uistr.RolePickerTitle) {
		t.Errorf("view missing title: %q", view)
	}
	if !strings.Contains(view, uistr.RoleFactHarnessOn) {
		t.Errorf("view missing harness fact: %q", view)
	}
	if !strings.Contains(view, uistr.RoleDefaultSuffix) {
		t.Errorf("view missing default marker: %q", view)
	}
}

func TestRolePickerOtherKeysIgnored(t *testing.T) {
	r := NewRolePicker("app/launch/role", roleOpts())
	before := r.cursor
	scr, cmd := r.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if cmd != nil {
		t.Errorf("x key produced a command, want nil")
	}
	if scr.(RolePicker).cursor != before {
		t.Errorf("x key moved cursor")
	}
}

func TestRolePickerEnterWithNoOptionsIsNoop(t *testing.T) {
	r := NewRolePicker("app/launch/role", nil)
	_, cmd := r.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("enter with no options produced a command, want nil")
	}
}
