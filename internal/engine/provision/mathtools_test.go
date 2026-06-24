package provision

import (
	"path/filepath"
	"testing"
)

func mathToolsScripts() map[string]string {
	return map[string]string{
		"venvProbe": mathVenvProbe,
		"venvMake":  mathVenvMake,
		"pipInst":   mathPipInst,
		"pipLink":   mathPipLink,
		"leanProbe": mathLeanProbe,
		"leanInst":  mathLeanInst,
		"coqProbe":  mathCoqProbe,
		"coqInst":   mathCoqInst,
	}
}

func writeRaidTools(t *testing.T, d Deps, tools string) {
	t.Helper()
	mustWrite(t, filepath.Join(d.Repo, "config", "loadouts", "raid.txt"),
		"effort max\nharness on\ntools "+tools+"\n")
}

func TestMathToolsNoopWhenLoadoutHasNoTools(t *testing.T) {
	d, f := testDeps(t)
	writeRaidTools(t, d, "")
	step := &mathToolsStep{d: d}
	if !checkStep(t, step) {
		t.Error("check should be true (no-op) when no math tools are requested")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := len(f.Calls()); n != 0 {
		t.Errorf("expected zero exec calls when no tools requested, got %d", n)
	}
}

func TestMathToolsNoopWhenLoadoutMissing(t *testing.T) {
	d, f := testDeps(t)
	step := &mathToolsStep{d: d}
	if !checkStep(t, step) {
		t.Error("check should be true (no-op) when loadout manifest is absent")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := len(f.Calls()); n != 0 {
		t.Errorf("expected zero exec calls when loadout absent, got %d", n)
	}
}

func TestMathToolsInstallsAllWhenMissing(t *testing.T) {
	d, f := testDeps(t)
	writeRaidTools(t, d, "sympy z3 lean coq")
	sc := mathToolsScripts()
	f.Expect(script(sc["venvProbe"]), "", errStub)
	f.Expect(script(sc["venvMake"]), "", nil)
	f.Expect(script(sc["pipInst"]), "", nil)
	f.Expect(script(sc["pipLink"]), "", nil)
	f.Expect(script(sc["leanProbe"]), "", errStub)
	f.Expect(script(sc["leanInst"]), "", nil)
	f.Expect(script(sc["coqProbe"]), "", errStub)
	f.Expect(script(sc["coqInst"]), "", nil)
	step := &mathToolsStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertScriptCalls(t, f.Calls(), []string{
		sc["venvProbe"], sc["venvMake"], sc["pipInst"], sc["pipLink"],
		sc["leanProbe"], sc["leanInst"], sc["coqProbe"], sc["coqInst"],
	})
}

func TestMathToolsSkipsInstalled(t *testing.T) {
	d, f := testDeps(t)
	writeRaidTools(t, d, "sympy z3 lean coq")
	sc := mathToolsScripts()
	f.Expect(script(sc["venvProbe"]), "ok", nil)
	f.Expect(script(sc["leanProbe"]), "ok", nil)
	f.Expect(script(sc["coqProbe"]), "ok", nil)
	step := &mathToolsStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertScriptCalls(t, f.Calls(), []string{sc["venvProbe"], sc["leanProbe"], sc["coqProbe"]})
}

func TestMathToolsCoqFailureTolerated(t *testing.T) {
	d, f := testDeps(t)
	writeRaidTools(t, d, "coq")
	sc := mathToolsScripts()
	f.Expect(script(sc["coqProbe"]), "", errStub)
	f.Expect(script(sc["coqInst"]), "", errStub)
	step := &mathToolsStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("coq install failure must not fail the step: %v", err)
	}
	f.Expect(script(sc["coqProbe"]), "", errStub)
	if !checkStep(t, step) {
		t.Error("check must be satisfied even when coq (best-effort) is still absent")
	}
}

func TestMathToolsSympyOnlyInstallsVenv(t *testing.T) {
	d, f := testDeps(t)
	writeRaidTools(t, d, "sympy")
	sc := mathToolsScripts()
	f.Expect(script(sc["venvProbe"]), "", errStub)
	f.Expect(script(sc["venvMake"]), "", nil)
	f.Expect(script(sc["pipInst"]), "", nil)
	f.Expect(script(sc["pipLink"]), "", nil)
	step := &mathToolsStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertScriptCalls(t, f.Calls(), []string{
		sc["venvProbe"], sc["venvMake"], sc["pipInst"], sc["pipLink"],
	})
}
