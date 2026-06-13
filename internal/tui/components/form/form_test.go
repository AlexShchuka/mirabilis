package form_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/form"
)

type driver struct {
	model  form.Model
	result *bus.ScreenResult
	popped bool
}

func newDriver(groups []form.Group) *driver {
	m := form.NewWizard(groups)
	m.SetSize(80, 24)
	d := &driver{model: m}
	d.send(m.Init()())
	return d
}

func (d *driver) send(msg tea.Msg) {
	queue := []tea.Msg{msg}
	for i := 0; i < 500 && len(queue) > 0; i++ {
		cur := queue[0]
		queue = queue[1:]
		switch v := cur.(type) {
		case bus.ScreenResult:
			r := v
			d.result = &r
			continue
		case bus.ScreenPop:
			d.popped = true
			continue
		}
		var cmd tea.Cmd
		d.model, cmd = d.model.Update(cur)
		queue = append(queue, drain(cmd)...)
	}
}

func (d *driver) key(k tea.KeyPressMsg) { d.send(k) }

func (d *driver) view() string { return stripANSI(d.model.View()) }

func (d *driver) footer() string {
	lines := strings.Split(strings.TrimRight(d.view(), "\n "), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

func drain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drain(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func threeGroups() []form.Group {
	return []form.Group{
		{Key: "stacks", Title: "Optional stacks", Description: "stacks-desc", Options: []string{"rust", "elixir"}},
		{Key: "plugins", Title: "Plugins", Description: "plugins-desc", Options: []string{"alpha", "beta"}, Selected: []string{"alpha", "beta"}},
		{Key: "skills", Title: "Optional skills", Description: "skills-desc", Options: []string{"writer"}},
	}
}

var (
	enter    = tea.KeyPressMsg{Code: tea.KeyEnter}
	shiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	toggle   = tea.KeyPressMsg{Code: 'x', Text: "x"}
)

func TestWizardShowsFirstGroupWithDescription(t *testing.T) {
	d := newDriver(threeGroups())
	v := d.view()
	if !strings.Contains(v, "Optional stacks") || !strings.Contains(v, "stacks-desc") {
		t.Fatalf("first group title/description not rendered:\n%s", v)
	}
	if strings.Contains(v, "plugins-desc") || strings.Contains(v, "skills-desc") {
		t.Fatalf("only the active group must render (Слои single-group), got:\n%s", v)
	}
}

func TestWizardForwardNavRevealsNextGroup(t *testing.T) {
	d := newDriver(threeGroups())
	d.key(enter)
	if v := d.view(); !strings.Contains(v, "plugins-desc") || strings.Contains(v, "stacks-desc") {
		t.Fatalf("enter did not advance to plugins group:\n%s", v)
	}
	d.key(enter)
	if v := d.view(); !strings.Contains(v, "skills-desc") || strings.Contains(v, "plugins-desc") {
		t.Fatalf("enter did not advance to skills group:\n%s", v)
	}
}

func TestWizardBackNavReturnsToPreviousGroup(t *testing.T) {
	d := newDriver(threeGroups())
	d.key(enter)
	if v := d.view(); !strings.Contains(v, "plugins-desc") {
		t.Fatalf("expected plugins group after enter:\n%s", v)
	}
	d.key(shiftTab)
	if v := d.view(); !strings.Contains(v, "stacks-desc") || strings.Contains(v, "plugins-desc") {
		t.Fatalf("shift+tab did not return to stacks group:\n%s", v)
	}
}

func TestWizardBackAffordanceShownOnNonFirstGroup(t *testing.T) {
	d := newDriver(threeGroups())
	if f := d.footer(); strings.Contains(f, "back") {
		t.Fatalf("first group must not advertise back, footer=%q", f)
	}
	d.key(enter)
	f := d.footer()
	if !strings.Contains(f, "shift+tab") || !strings.Contains(f, "back") {
		t.Fatalf("back affordance missing on second group, footer=%q", f)
	}
}

func TestWizardSubmitReturnsAllKeyedChoices(t *testing.T) {
	d := newDriver(threeGroups())
	d.key(toggle)
	d.key(enter)
	d.key(toggle)
	d.key(enter)
	d.key(toggle)
	d.key(enter)
	if d.result == nil {
		t.Fatalf("submit did not emit bus.ScreenResult; footer=%q", d.footer())
	}
	if d.popped {
		t.Fatal("submit also emitted bus.ScreenPop")
	}
	got := d.result.Values
	if got == nil {
		t.Fatalf("ScreenResult.Values is nil, want keyed map")
	}
	want := map[string][]string{
		"stacks":  {"rust"},
		"plugins": {"beta"},
		"skills":  {"writer"},
	}
	for key, w := range want {
		if !equal(got[key], w) {
			t.Errorf("group %q choice = %v, want %v", key, got[key], w)
		}
	}
}

func TestWizardSubmitKeepsPreselected(t *testing.T) {
	d := newDriver([]form.Group{
		{Key: "plugins", Title: "Plugins", Description: "d", Options: []string{"alpha", "beta"}, Selected: []string{"beta"}},
	})
	d.key(enter)
	if d.result == nil {
		t.Fatal("submit did not emit bus.ScreenResult")
	}
	if got := d.result.Values["plugins"]; !equal(got, []string{"beta"}) {
		t.Fatalf("plugins = %v, want [beta]", got)
	}
}

func TestWizardEscEmitsScreenPop(t *testing.T) {
	d := newDriver(threeGroups())
	d.key(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !d.popped {
		t.Fatal("esc did not emit bus.ScreenPop")
	}
	if d.result != nil {
		t.Fatal("esc also emitted bus.ScreenResult")
	}
}

func TestWizardCtrlCEmitsScreenPop(t *testing.T) {
	d := newDriver(threeGroups())
	d.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !d.popped {
		t.Fatal("ctrl+c did not emit bus.ScreenPop")
	}
}

func TestWizardSingleGroupSubmits(t *testing.T) {
	d := newDriver([]form.Group{
		{Key: "stacks", Title: "Stacks", Description: "only", Options: []string{"rust", "elixir"}},
	})
	if f := d.footer(); strings.Contains(f, "back") {
		t.Fatalf("single group must not advertise back, footer=%q", f)
	}
	d.key(toggle)
	d.key(enter)
	if d.result == nil {
		t.Fatal("single-group submit did not emit bus.ScreenResult")
	}
	if got := d.result.Values["stacks"]; !equal(got, []string{"rust"}) {
		t.Fatalf("stacks = %v, want [rust]", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
