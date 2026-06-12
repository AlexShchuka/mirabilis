package provision

import (
	"errors"
	"testing"
)

func TestRTKStepRunsInitWhenHookAbsent(t *testing.T) {
	d, f := testDeps(t)
	mustWriteJSON(t, d.settingsPath(), map[string]any{"hooks": map[string]any{}})
	f.Expect([]string{"rtk", "--version"}, "rtk 0.1", nil)
	f.Expect([]string{"timeout", "60", "rtk", "init", "-g", "--auto-patch"}, "", nil)
	if err := runStep(t, &rtkStep{d: d}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("init not invoked: %d stubs unused", n)
	}
}

func TestRTKStepNoopWithoutRTK(t *testing.T) {
	d, f := testDeps(t)
	f.Expect([]string{"rtk", "--version"}, "", errors.New("missing"))
	f.Expect([]string{"rtk", "--version"}, "", errors.New("missing"))
	step := &rtkStep{d: d}
	if !checkStep(t, step) {
		t.Error("check should be true when rtk is absent")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := len(f.Calls()); got != 2 {
		t.Errorf("calls = %d, want 2 probes only", got)
	}
}
