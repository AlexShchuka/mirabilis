package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
)

func TestCreateMarkerStep(t *testing.T) {
	d, _ := testDeps(t)
	step := &createMarkerStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false before the marker exists")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), harness.CreateMarkerName))
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
	d, _ := testDeps(t)
	d.Fingerprint = "v1.2.3"
	d.SessionKey = "session"
	step := &startMarkerStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false before the marker exists")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), harness.StartMarkerName))
	if err != nil {
		t.Fatal(err)
	}
	want := harness.StartMarkerHash("v1.2.3", "session")
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
