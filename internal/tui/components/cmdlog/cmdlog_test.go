package cmdlog

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func keyUp() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyUp}
}

func keyDown() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyDown}
}

func filled(n int) Model {
	m := New()
	m.SetSize(40, 5)
	for i := 0; i < n; i++ {
		m, _ = m.Update(bus.StepEvent{Step: "s", Kind: bus.StepLine, Line: fmt.Sprintf("line %d", i)})
	}
	return m
}

func TestStepStartedRendersArgv(t *testing.T) {
	m := New()
	m.SetSize(40, 5)
	m, _ = m.Update(bus.StepEvent{Step: "container", Kind: bus.StepStarted, Argv: []string{"docker", "compose", "up", "-d"}})
	view := plain(m.View())
	if !strings.Contains(view, "+ docker compose up -d") {
		t.Errorf("View() = %q, want argv line", view)
	}
}

func TestStepStartedWithoutArgvAddsNothing(t *testing.T) {
	m := New()
	m.SetSize(40, 5)
	m, _ = m.Update(bus.StepEvent{Step: "container", Kind: bus.StepStarted})
	if got := m.Lines(); len(got) != 0 {
		t.Errorf("Lines() = %v, want empty", got)
	}
}

func TestStepLineAdds(t *testing.T) {
	m := New()
	m.SetSize(40, 5)
	m, _ = m.Update(bus.StepEvent{Step: "container", Kind: bus.StepLine, Line: "#12 RUN go install"})
	if !strings.Contains(plain(m.View()), "#12 RUN go install") {
		t.Errorf("View() = %q, want step line", m.View())
	}
}

func TestAutoscroll(t *testing.T) {
	m := filled(10)
	if !m.Following() {
		t.Fatal("Following() = false, want autoscroll on")
	}
	view := plain(m.View())
	if !strings.Contains(view, "line 9") {
		t.Errorf("View() = %q, want newest line visible", view)
	}
	if strings.Contains(view, "line 0\n") {
		t.Errorf("View() = %q, oldest line should be scrolled out", view)
	}
}

func TestScrollUpSuspendsFollow(t *testing.T) {
	m := filled(10)
	m.Focus()
	m, _ = m.Update(keyUp())
	if m.Following() {
		t.Fatal("Following() = true after scrolling up")
	}
	m, _ = m.Update(bus.StepEvent{Step: "s", Kind: bus.StepLine, Line: "line 10"})
	if strings.Contains(plain(m.View()), "line 10") {
		t.Errorf("View() = %q, new line must not force a jump while scrolled up", m.View())
	}
}

func TestScrollBackToBottomResumesFollow(t *testing.T) {
	m := filled(10)
	m.Focus()
	m, _ = m.Update(keyUp())
	for i := 0; i < 5; i++ {
		m, _ = m.Update(keyDown())
	}
	if !m.Following() {
		t.Fatal("Following() = false after returning to bottom")
	}
	m, _ = m.Update(bus.StepEvent{Step: "s", Kind: bus.StepLine, Line: "line 10"})
	if !strings.Contains(plain(m.View()), "line 10") {
		t.Errorf("View() = %q, want autoscroll resumed", m.View())
	}
}

func TestKeysIgnoredWhenBlurred(t *testing.T) {
	m := filled(10)
	if m.Focused() {
		t.Fatal("Focused() = true, want false by default")
	}
	m, _ = m.Update(keyUp())
	if !m.Following() {
		t.Fatal("Following() = false, keys must be ignored without focus")
	}
	m.Focus()
	if !m.Focused() {
		t.Fatal("Focused() = false after Focus()")
	}
	m.Blur()
	if m.Focused() {
		t.Fatal("Focused() = true after Blur()")
	}
}

func TestViewTitleRule(t *testing.T) {
	m := New()
	m.SetSize(40, 5)
	first := strings.Split(plain(m.View()), "\n")[0]
	if !strings.Contains(first, "─ commands ─") {
		t.Errorf("title rule = %q, want commands rule", first)
	}
}

func TestRingBufferCap(t *testing.T) {
	const n = 20000
	m := New()
	for i := range n {
		m.add(fmt.Sprintf("line %d", i))
	}
	lines := m.Lines()
	if len(lines) > maxLines {
		t.Fatalf("len(lines) = %d after %d adds, want ≤ %d", len(lines), n, maxLines)
	}
	newest := fmt.Sprintf("line %d", n-1)
	if len(lines) == 0 || lines[len(lines)-1] != newest {
		last := ""
		if len(lines) > 0 {
			last = lines[len(lines)-1]
		}
		t.Errorf("newest line = %q, want %q", last, newest)
	}
}
