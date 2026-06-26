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
		{Key: "spark", Effort: "medium"},
		{Key: "drift", Effort: "high"},
		{Key: "orbit", Effort: "max"},
		{Key: "forge", Effort: "xhigh", Batch: true, Default: true},
		{Key: "nova", Effort: "max", Batch: true},
	}
}

func sized(r RolePicker, w, h int) RolePicker {
	scr, _ := r.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return scr.(RolePicker)
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
	if r.cursor != 3 {
		t.Errorf("cursor = %d, want 3 (forge is default)", r.cursor)
	}
}

func TestRolePickerCursorStartsAtZeroWithoutDefault(t *testing.T) {
	r := NewRolePicker("app/launch/role", []RoleOption{{Key: "spark"}, {Key: "drift"}})
	if r.cursor != 0 {
		t.Errorf("cursor = %d, want 0", r.cursor)
	}
}

func TestRolePickerHorizontalNavigation(t *testing.T) {
	r := sized(NewRolePicker("app/launch/role", roleOpts()), 100, 30)

	scr, _ := r.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	r = scr.(RolePicker)
	if r.cursor != 2 {
		t.Errorf("after left: cursor = %d, want 2", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	r = scr.(RolePicker)
	scr, _ = r.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	r = scr.(RolePicker)
	if r.cursor != 4 {
		t.Errorf("after l l: cursor = %d, want 4", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	r = scr.(RolePicker)
	if r.cursor != 4 {
		t.Errorf("right at end: cursor = %d, want 4 (clamped)", r.cursor)
	}
}

func TestRolePickerVerticalNavigationMovesByColumns(t *testing.T) {
	r := sized(NewRolePicker("app/launch/role", roleOpts()), 60, 30)
	if cols := r.columns(); cols != 2 {
		t.Fatalf("columns() = %d, want 2 at width 60", cols)
	}

	scr, _ := r.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	r = scr.(RolePicker)
	if r.cursor != 1 {
		t.Errorf("after up from 3: cursor = %d, want 1", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	r = scr.(RolePicker)
	if r.cursor != 3 {
		t.Errorf("after down from 1: cursor = %d, want 3", r.cursor)
	}

	scr, _ = r.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	r = scr.(RolePicker)
	scr, _ = r.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	r = scr.(RolePicker)
	if r.cursor != 1 {
		t.Errorf("up at top row clamps: cursor = %d, want 1", r.cursor)
	}
}

func TestRolePickerFallsBackToSingleColumn(t *testing.T) {
	r := sized(NewRolePicker("app/launch/role", roleOpts()), 10, 30)
	if cols := r.columns(); cols != 1 {
		t.Errorf("columns() = %d at narrow width, want 1", cols)
	}
	scr, _ := r.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	r = scr.(RolePicker)
	if r.cursor != 2 {
		t.Errorf("up in single column from 3: cursor = %d, want 2", r.cursor)
	}
}

func TestRolePickerEnterEmitsResultWithSelectedKey(t *testing.T) {
	r := sized(NewRolePicker("app/launch/role", roleOpts()), 100, 30)
	scr, _ := r.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	r = scr.(RolePicker)
	_, cmd := r.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := emit(cmd)
	res, ok := msg.(bus.ScreenResult)
	if !ok {
		t.Fatalf("enter: got %T, want bus.ScreenResult", msg)
	}
	if res.Value != "orbit" {
		t.Errorf("ScreenResult.Value = %v, want orbit", res.Value)
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
	if res.Value != "forge" {
		t.Errorf("ScreenResult.Value = %v, want forge", res.Value)
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

func TestRolePickerRendersAllParties(t *testing.T) {
	r := sized(NewRolePicker("app/launch/role", roleOpts()), 100, 30)
	view := plain(r.View())
	for _, key := range []string{"spark", "drift", "orbit", "forge", "nova"} {
		if !strings.Contains(view, key) {
			t.Errorf("view missing party %q: %q", key, view)
		}
	}
	if !strings.Contains(view, uistr.RolePickerTitle) {
		t.Errorf("view missing title: %q", view)
	}
	if !strings.Contains(view, uistr.RoleFactFleet) {
		t.Errorf("view missing fleet fact: %q", view)
	}
	if !strings.Contains(view, uistr.RoleFactSolo) {
		t.Errorf("view missing solo fact: %q", view)
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
