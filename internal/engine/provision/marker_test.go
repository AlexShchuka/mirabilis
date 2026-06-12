package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartMarkerHashFormulaStable(t *testing.T) {
	const golden = "2476eea9945737aec76cb5c18e3acf117384f883949438952343fbd4e428953f"
	if got := StartMarkerHash("v1.2.3", "session"); got != golden {
		t.Errorf("StartMarkerHash(v1.2.3, session) = %s, want %s", got, golden)
	}
	if StartMarkerHash("v1.2.3", "other") == golden {
		t.Error("hash must change with the session key")
	}
	if StartMarkerHash("v9.9.9", "session") == golden {
		t.Error("hash must change with the fingerprint")
	}
}

func TestCreateMarkerStep(t *testing.T) {
	d, _ := testDeps(t)
	step := &createMarkerStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false before the marker exists")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), CreateMarkerName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ok\n" {
		t.Errorf("marker content = %q, want ok", data)
	}
	if !checkStep(t, step) {
		t.Error("check should be true after run")
	}
}

func TestStartMarkerStep(t *testing.T) {
	t.Setenv("MIRABILIS_VERSION", "v1.2.3")
	d, _ := testDeps(t)
	d.SessionKey = "session"
	step := &startMarkerStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false before the marker exists")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), StartMarkerName))
	if err != nil {
		t.Fatal(err)
	}
	want := StartMarkerHash("v1.2.3", "session")
	if strings.TrimSpace(string(data)) != want {
		t.Errorf("marker content = %q, want %s", data, want)
	}
	if !checkStep(t, step) {
		t.Error("check should be true after run")
	}
	rotated := step.d
	rotated.SessionKey = "rotated"
	if checkStep(t, &startMarkerStep{d: rotated}) {
		t.Error("check should be false after the session key rotates")
	}
}
