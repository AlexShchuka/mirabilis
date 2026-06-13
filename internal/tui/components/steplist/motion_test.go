package steplist

import (
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/bus"
)

func threeRows() []StepRow {
	return []StepRow{
		{Name: "a", Title: "A"},
		{Name: "b", Title: "B"},
		{Name: "c", Title: "C"},
	}
}

func TestRunningGlyphStaticUnderReducedMotion(t *testing.T) {
	t.Setenv("NO_ANIMATE", "1")
	m := New([]StepRow{{Name: "a", Title: "A", State: StateRunning}})
	if cmd := m.Init(); cmd != nil {
		t.Error("Init returned a spinner tick under reduced motion, want nil")
	}
	view := plain(m.View())
	if !strings.Contains(view, "▸") {
		t.Errorf("running row lacks the static glyph under reduced motion:\n%s", view)
	}
}

func TestProgressBarShowsNOfTotal(t *testing.T) {
	t.Setenv("NO_ANIMATE", "1")
	m := New(threeRows())
	m, _ = m.Update(bus.StepEvent{Step: "a", Kind: bus.StepDone})
	m, _ = m.Update(bus.StepEvent{Step: "b", Kind: bus.StepFailed})
	view := plain(m.View())
	if !strings.Contains(view, "2/3") {
		t.Errorf("progress bar missing 2/3 count after two completions:\n%s", view)
	}
}

func TestProgressReducedMotionNoTickFullValue(t *testing.T) {
	t.Setenv("NO_ANIMATE", "1")
	m := New(threeRows())
	m, cmd := m.Update(bus.StepEvent{Step: "a", Kind: bus.StepDone})
	if cmd != nil {
		t.Error("step completion under reduced motion returned a tick cmd, want nil (WCAG 2.3.3)")
	}
	if m.pos != m.target {
		t.Errorf("pos %.3f != target %.3f under reduced motion; value must jump, not tween", m.pos, m.target)
	}
	if m.animate {
		t.Error("animate = true under reduced motion, want false")
	}
}

func TestProgressTickAdvancesStateNotView(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("NO_ANIMATE", "")
	t.Setenv("ACCESSIBLE", "")
	m := New(threeRows())
	m, cmd := m.Update(bus.StepEvent{Step: "a", Kind: bus.StepDone})
	if cmd == nil {
		t.Fatal("step completion with motion returned nil cmd, want a progress tick")
	}
	if !m.animate {
		t.Fatal("animate = false after completion with motion on, want true")
	}
	before := m.pos
	beforeView := m.View()
	if m.View() != beforeView {
		t.Error("View() is not pure: two renders of the same model differ (B6)")
	}
	if m.pos != before {
		t.Error("View() advanced the spring; motion must advance only in the tick handler, not View (B6)")
	}
	m2, _ := m.Update(progressTickMsg{})
	if m2.pos == before {
		t.Error("progress tick did not advance pos; spring must update in the tick handler")
	}
}
