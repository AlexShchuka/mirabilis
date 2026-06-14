package steplist

import (
	"regexp"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/bus"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func rows() []StepRow {
	return []StepRow{
		{Name: "preflight", Title: "Preflight"},
		{Name: "container", Title: "Container"},
	}
}

func TestTransitions(t *testing.T) {
	tests := []struct {
		name       string
		events     []bus.StepEvent
		wantState  State
		wantDetail string
	}{
		{
			name:      "started",
			events:    []bus.StepEvent{{Step: "preflight", Kind: bus.StepStarted}},
			wantState: StateRunning,
		},
		{
			name: "line updates detail",
			events: []bus.StepEvent{
				{Step: "preflight", Kind: bus.StepStarted},
				{Step: "preflight", Kind: bus.StepLine, Line: "docker 28.1"},
			},
			wantState:  StateRunning,
			wantDetail: "docker 28.1",
		},
		{
			name: "line tail wins",
			events: []bus.StepEvent{
				{Step: "preflight", Kind: bus.StepLine, Line: "first"},
				{Step: "preflight", Kind: bus.StepLine, Line: "second"},
			},
			wantState:  StatePending,
			wantDetail: "second",
		},
		{
			name: "done",
			events: []bus.StepEvent{
				{Step: "preflight", Kind: bus.StepStarted},
				{Step: "preflight", Kind: bus.StepDone, Line: "compose ok"},
			},
			wantState:  StateDone,
			wantDetail: "compose ok",
		},
		{
			name: "done keeps last detail",
			events: []bus.StepEvent{
				{Step: "preflight", Kind: bus.StepLine, Line: "tail"},
				{Step: "preflight", Kind: bus.StepDone},
			},
			wantState:  StateDone,
			wantDetail: "tail",
		},
		{
			name:       "failed",
			events:     []bus.StepEvent{{Step: "preflight", Kind: bus.StepFailed, Line: "boom"}},
			wantState:  StateFailed,
			wantDetail: "boom",
		},
		{
			name:      "skipped",
			events:    []bus.StepEvent{{Step: "preflight", Kind: bus.StepSkipped}},
			wantState: StateSkipped,
		},
		{
			name:       "waiting default detail",
			events:     []bus.StepEvent{{Step: "preflight", Kind: bus.StepWaiting}},
			wantState:  StateWaiting,
			wantDetail: "waiting",
		},
		{
			name:       "waiting custom detail",
			events:     []bus.StepEvent{{Step: "preflight", Kind: bus.StepWaiting, Line: "confirm in browser"}},
			wantState:  StateWaiting,
			wantDetail: "confirm in browser",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(rows())
			for _, ev := range tt.events {
				m, _ = m.Update(ev)
			}
			got := m.Rows()[0]
			if got.State != tt.wantState {
				t.Errorf("state = %v, want %v", got.State, tt.wantState)
			}
			if got.Detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", got.Detail, tt.wantDetail)
			}
			if other := m.Rows()[1]; other.State != StatePending || other.Detail != "" {
				t.Errorf("untouched row changed: %+v", other)
			}
		})
	}
}

func TestUnknownStepIgnored(t *testing.T) {
	m := New(rows())
	m, _ = m.Update(bus.StepEvent{Step: "nope", Kind: bus.StepDone})
	for _, r := range m.Rows() {
		if r.State != StatePending {
			t.Errorf("row %q state = %v, want pending", r.Name, r.State)
		}
	}
}

func TestStartedReturnsTick(t *testing.T) {
	m := New(rows())
	m, cmd := m.Update(bus.StepEvent{Step: "preflight", Kind: bus.StepStarted})
	if cmd == nil {
		t.Fatal("started should return a spinner tick cmd")
	}
	if !m.anyRunning() {
		t.Fatal("anyRunning() = false after started")
	}
}

func TestViewGlyphs(t *testing.T) {
	m := New([]StepRow{
		{Name: "a", Title: "Done step", State: StateDone},
		{Name: "b", Title: "Failed step", State: StateFailed},
		{Name: "c", Title: "Pending step"},
		{Name: "d", Title: "Skipped step", State: StateSkipped},
		{Name: "e", Title: "Running step", State: StateRunning},
	})
	view := plain(m.View())
	lines := rowLines(t, m, view)
	wantPrefix := []string{" ✔ ", " ✖ ", " · ", " − ", " ⠋ "}
	for i, want := range wantPrefix {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("line %d = %q, want prefix %q", i, lines[i], want)
		}
	}
}

func rowLines(t *testing.T, m Model, view string) []string {
	t.Helper()
	lines := strings.Split(view, "\n")
	if m.progressView() != "" {
		if len(lines) < 1 {
			t.Fatal("view has no lines")
		}
		return lines[1:]
	}
	return lines
}

func TestWaitingGlyphDistinct(t *testing.T) {
	m := New([]StepRow{
		{Name: "a", Title: "Pending step"},
		{Name: "b", Title: "Waiting step", State: StateWaiting},
	})
	view := plain(m.View())
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("view has %d lines, want at least 2", len(lines))
	}
	if strings.HasPrefix(lines[1], " · ") {
		t.Error("waiting row uses pending glyph '·'; expected a distinct waiting glyph")
	}
	if !strings.HasPrefix(lines[1], " ? ") {
		t.Errorf("waiting row = %q, want prefix \" ? \"", lines[1])
	}
}

func TestSetSizeReflowsRowsToWidth(t *testing.T) {
	m := New([]StepRow{
		{Name: "a", Title: "Container", Detail: "a very long detail line that would overflow a narrow terminal area", State: StateRunning},
	})
	m.SetSize(20, 10)
	view := plain(m.View())
	for i, line := range strings.Split(view, "\n") {
		if w := len([]rune(line)); w > 20 {
			t.Errorf("line %d width = %d runes, want <= 20 (must reflow, not overflow): %q", i, w, line)
		}
	}
}

func TestSetSizeClampsHeightWithOverflowAffordance(t *testing.T) {
	rows := make([]StepRow, 0, 8)
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		rows = append(rows, StepRow{Name: n, Title: "Step " + n})
	}
	m := New(rows)
	m.SetSize(40, 4)
	view := plain(m.View())
	lines := strings.Split(view, "\n")
	if len(lines) > 4 {
		t.Fatalf("clamped view has %d lines, want <= 4", len(lines))
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, "more") {
		t.Errorf("overflow affordance missing; last line = %q, want an \"+N more\" indicator", last)
	}
}

func TestNoSizeRendersAllRows(t *testing.T) {
	rows := make([]StepRow, 0, 8)
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		rows = append(rows, StepRow{Name: n, Title: "Step " + n})
	}
	m := New(rows)
	view := plain(m.View())
	if got := len(strings.Split(view, "\n")); got != 8 {
		t.Errorf("unsized view has %d lines, want 8 (no clamp without SetSize)", got)
	}
}
