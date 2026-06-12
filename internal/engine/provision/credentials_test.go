package provision

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialsStepRemovesLeakedFile(t *testing.T) {
	d, _ := testDeps(t)
	step := &credentialsStep{d: d}
	path := filepath.Join(d.claudeDir(), credentialsFileName)

	if !checkStep(t, step) {
		t.Error("check should be true when no credentials file exists")
	}

	if err := os.MkdirAll(d.claudeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"claudeAiOauth":{"accessToken":"sk-ant-oat01-LEAKED"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if checkStep(t, step) {
		t.Error("check should be false while the credentials file exists")
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("credentials file still present after run: %v", err)
	}
	if !checkStep(t, step) {
		t.Error("check should be true after removal")
	}
}

func TestCredentialsStepIdempotentOnMissingFile(t *testing.T) {
	d, _ := testDeps(t)
	step := &credentialsStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run on missing file: %v", err)
	}
	if err := runStep(t, step); err != nil {
		t.Fatalf("repeat run: %v", err)
	}
}

func TestCredentialsStepIsNotOptional(t *testing.T) {
	step := &credentialsStep{}
	if step.Meta().Optional {
		t.Fatal("credentials guard must be a hard gate (I1), not an optional step")
	}
}
