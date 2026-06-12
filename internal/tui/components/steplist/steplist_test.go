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
	lines := strings.Split(view, "\n")
	wantPrefix := []string{" ✔ ", " ✖ ", " · ", " − ", " ⠋ "}
	for i, want := range wantPrefix {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("line %d = %q, want prefix %q", i, lines[i], want)
		}
	}
}
