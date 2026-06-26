package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func writeLoadoutManifest(t *testing.T, repo, name, content string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, "config", "loadouts", name+".txt"), content)
}

func readHarnessPref(t *testing.T, d Deps) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), fileHarness))
	if err != nil {
		t.Fatalf("read %s: %v", fileHarness, err)
	}
	return strings.TrimSpace(string(data))
}

func TestLoadoutStepDefaultFallback(t *testing.T) {
	d, _ := testDeps(t)
	step := &loadoutStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false before loadout file exists")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), fileLoadout))
	if err != nil {
		t.Fatalf("loadout file not written: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != config.DefaultLoadout {
		t.Errorf("loadout file = %q, want %q", got, config.DefaultLoadout)
	}
	if !checkStep(t, step) {
		t.Error("check should be true after run")
	}
}

func TestLoadoutStepHarnessOffWritesSkip(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "pvp", "effort max\nharness off\n")
	if err := config.WriteLoadout(d.Repo, "pvp"); err != nil {
		t.Fatal(err)
	}
	step := &loadoutStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := readHarnessPref(t, d); got != harnessSkip {
		t.Errorf(".mirabilis-harness = %q, want %q (pvp has harness off)", got, harnessSkip)
	}
	if !checkStep(t, step) {
		t.Error("check should be true after run")
	}
}

func TestLoadoutStepHarnessOnWritesInstall(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "raid", "effort max\nharness on\n")
	if err := config.WriteLoadout(d.Repo, "raid"); err != nil {
		t.Fatal(err)
	}
	step := &loadoutStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := readHarnessPref(t, d); got != harnessInstall {
		t.Errorf(".mirabilis-harness = %q, want %q (raid has harness on)", got, harnessInstall)
	}
}

func TestLoadoutStepGrindHarnessOff(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "grind", "effort xhigh\nharness off\n")
	if err := config.WriteLoadout(d.Repo, "grind"); err != nil {
		t.Fatal(err)
	}
	step := &loadoutStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := readHarnessPref(t, d); got != harnessSkip {
		t.Errorf(".mirabilis-harness = %q, want %q (grind has harness off)", got, harnessSkip)
	}
}

func TestLoadoutStepIdempotent(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "pvp", "effort max\nharness off\n")
	if err := config.WriteLoadout(d.Repo, "pvp"); err != nil {
		t.Fatal(err)
	}
	step := &loadoutStep{d: d}
	pipeline.Contract(t, step, nil)
}

func TestLoadoutDesiredPrefersEnv(t *testing.T) {
	d, _ := testDeps(t)
	if err := config.WriteLoadout(d.Repo, "raid"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(loadoutEnvVar, "pvp")
	step := &loadoutStep{d: d}
	if got := step.desired(); got != "pvp" {
		t.Errorf("desired() = %q, want pvp (env overrides .env LOADOUT)", got)
	}
}

func TestLoadoutDesiredIgnoresDefaultEnv(t *testing.T) {
	d, _ := testDeps(t)
	if err := config.WriteLoadout(d.Repo, "raid"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(loadoutEnvVar, "default")
	step := &loadoutStep{d: d}
	if got := step.desired(); got != "raid" {
		t.Errorf("desired() = %q, want raid (env=default must not override)", got)
	}
}

func TestDepsLoadoutPrefersEnvManifest(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "raid", "effort medium\nharness on\n")
	writeLoadoutManifest(t, d.Repo, "pvp", "effort max\nharness off\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileLoadout), "raid\n")
	t.Setenv(loadoutEnvVar, "pvp")
	lo, ok := d.loadout()
	if !ok {
		t.Fatal("loadout() not ok")
	}
	if lo.Name != "pvp" || lo.Effort != "max" || lo.Harness {
		t.Errorf("loadout() = %+v, want pvp/max/harness-off from env", lo)
	}
}

func TestLoadoutStepSwitchLoadoutUpdatesHarness(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "raid", "effort max\nharness on\n")
	writeLoadoutManifest(t, d.Repo, "pvp", "effort max\nharness off\n")

	if err := config.WriteLoadout(d.Repo, "raid"); err != nil {
		t.Fatal(err)
	}
	step := &loadoutStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run raid: %v", err)
	}
	if got := readHarnessPref(t, d); got != harnessInstall {
		t.Errorf("after raid: .mirabilis-harness = %q, want install", got)
	}

	if err := config.WriteLoadout(d.Repo, "pvp"); err != nil {
		t.Fatal(err)
	}
	step2 := &loadoutStep{d: d}
	if checkStep(t, step2) {
		t.Error("check should be false after switching loadout to pvp")
	}
	if err := runStep(t, step2); err != nil {
		t.Fatalf("run pvp: %v", err)
	}
	if got := readHarnessPref(t, d); got != harnessSkip {
		t.Errorf("after pvp: .mirabilis-harness = %q, want skip", got)
	}
}
