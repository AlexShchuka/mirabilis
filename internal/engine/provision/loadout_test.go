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

func TestLoadoutStepWritesNoHarnessFile(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "forge", "effort xhigh\nbatch on\n")
	if err := config.WriteLoadout(d.Repo, "forge"); err != nil {
		t.Fatal(err)
	}
	step := &loadoutStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(d.claudeDir(), ".mirabilis-harness")); !os.IsNotExist(err) {
		t.Errorf(".mirabilis-harness should not be written (harness is no longer a loadout axis): err=%v", err)
	}
}

func TestHarnessChoiceAlwaysInstall(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "spark", "effort medium\npacks core\n")
	if err := config.WriteLoadout(d.Repo, "spark"); err != nil {
		t.Fatal(err)
	}
	if got := d.harnessChoice(); got != harnessInstall {
		t.Errorf("harnessChoice() = %q, want %q (harness always installs)", got, harnessInstall)
	}
}

func TestLoadoutStepIdempotent(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "drift", "effort high\npacks core\n")
	if err := config.WriteLoadout(d.Repo, "drift"); err != nil {
		t.Fatal(err)
	}
	step := &loadoutStep{d: d}
	pipeline.Contract(t, step, nil)
}

func TestLoadoutDesiredPrefersEnv(t *testing.T) {
	d, _ := testDeps(t)
	if err := config.WriteLoadout(d.Repo, "forge"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(loadoutEnvVar, "spark")
	step := &loadoutStep{d: d}
	if got := step.desired(); got != "spark" {
		t.Errorf("desired() = %q, want spark (env overrides .env LOADOUT)", got)
	}
}

func TestLoadoutDesiredIgnoresDefaultEnv(t *testing.T) {
	d, _ := testDeps(t)
	if err := config.WriteLoadout(d.Repo, "forge"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(loadoutEnvVar, "default")
	step := &loadoutStep{d: d}
	if got := step.desired(); got != "forge" {
		t.Errorf("desired() = %q, want forge (env=default must not override)", got)
	}
}

func TestDepsLoadoutPrefersEnvManifest(t *testing.T) {
	d, _ := testDeps(t)
	writeLoadoutManifest(t, d.Repo, "forge", "effort medium\nbatch on\n")
	writeLoadoutManifest(t, d.Repo, "spark", "effort high\npacks core\n")
	mustWrite(t, filepath.Join(d.claudeDir(), fileLoadout), "forge\n")
	t.Setenv(loadoutEnvVar, "spark")
	lo, ok := d.loadout()
	if !ok {
		t.Fatal("loadout() not ok")
	}
	if lo.Name != "spark" || lo.Effort != "high" {
		t.Errorf("loadout() = %+v, want spark/high from env", lo)
	}
}
