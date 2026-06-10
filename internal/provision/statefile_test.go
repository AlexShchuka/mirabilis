package provision

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestReadHarnessChoiceContainer_Skip(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "skip\n", nil
		},
	}
	got := ReadHarnessChoiceContainer(context.Background(), r)
	if got != HarnessSkip {
		t.Errorf("ReadHarnessChoiceContainer = %q, want %q", got, HarnessSkip)
	}
}

func TestReadHarnessChoiceContainer_Install(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "install\n", nil
		},
	}
	got := ReadHarnessChoiceContainer(context.Background(), r)
	if got != HarnessInstall {
		t.Errorf("ReadHarnessChoiceContainer = %q, want %q", got, HarnessInstall)
	}
}

func TestReadHarnessChoiceContainer_UsesCorrectFile(t *testing.T) {
	var capturedScript string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			capturedScript = strings.Join(args, " ")
			return "", nil
		},
	}
	ReadHarnessChoiceContainer(context.Background(), r)
	if !strings.Contains(capturedScript, FileHarness) {
		t.Errorf("ReadHarnessChoiceContainer script = %q, want to reference %s", capturedScript, FileHarness)
	}
}

func TestWriteHarnessChoiceContainer_WritesSkip(t *testing.T) {
	var capturedScript string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			capturedScript = strings.Join(args, " ")
			return "", nil
		},
	}
	if err := WriteHarnessChoiceContainer(context.Background(), r, HarnessSkip); err != nil {
		t.Fatalf("WriteHarnessChoiceContainer = %v, want nil", err)
	}
	if !strings.Contains(capturedScript, HarnessSkip) {
		t.Errorf("WriteHarnessChoiceContainer script = %q, want to contain %s", capturedScript, HarnessSkip)
	}
	if !strings.Contains(capturedScript, FileHarness) {
		t.Errorf("WriteHarnessChoiceContainer script = %q, want to reference %s", capturedScript, FileHarness)
	}
}

func TestWriteHarnessChoiceContainer_WritesInstall(t *testing.T) {
	var capturedScript string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			capturedScript = strings.Join(args, " ")
			return "", nil
		},
	}
	if err := WriteHarnessChoiceContainer(context.Background(), r, HarnessInstall); err != nil {
		t.Fatalf("WriteHarnessChoiceContainer = %v, want nil", err)
	}
	if !strings.Contains(capturedScript, HarnessInstall) {
		t.Errorf("WriteHarnessChoiceContainer script = %q, want to contain %s", capturedScript, HarnessInstall)
	}
}

func TestReadDisabledPluginsContainer_ReturnsRaw(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "plugin-x\nplugin-y\n", nil
		},
	}
	got := ReadDisabledPluginsContainer(context.Background(), r)
	if !strings.Contains(got, "plugin-x") || !strings.Contains(got, "plugin-y") {
		t.Errorf("ReadDisabledPluginsContainer = %q, want plugin-x and plugin-y", got)
	}
}

func TestReadDisabledPluginsContainer_UsesCorrectFile(t *testing.T) {
	var capturedScript string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			capturedScript = strings.Join(args, " ")
			return "", nil
		},
	}
	ReadDisabledPluginsContainer(context.Background(), r)
	if !strings.Contains(capturedScript, FilePluginsDisabled) {
		t.Errorf("ReadDisabledPluginsContainer script = %q, want to reference %s", capturedScript, FilePluginsDisabled)
	}
}

func TestWriteDisabledPluginsContainer_PassesContent(t *testing.T) {
	content := "plugin-a\nplugin-b"
	var capturedArgs []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			capturedArgs = args
			return "", nil
		},
	}
	if err := WriteDisabledPluginsContainer(context.Background(), r, content); err != nil {
		t.Fatalf("WriteDisabledPluginsContainer = %v, want nil", err)
	}
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, FilePluginsDisabled) {
		t.Errorf("WriteDisabledPluginsContainer args = %v, want to reference %s", capturedArgs, FilePluginsDisabled)
	}
	if !strings.Contains(joined, content) {
		t.Errorf("WriteDisabledPluginsContainer args = %v, want content %q", capturedArgs, content)
	}
}

func TestReadSkillsContainer_ReturnsRaw(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "owner/skill-a\nowner/skill-b\n", nil
		},
	}
	got := ReadSkillsContainer(context.Background(), r)
	if !strings.Contains(got, "owner/skill-a") || !strings.Contains(got, "owner/skill-b") {
		t.Errorf("ReadSkillsContainer = %q, want owner/skill-a and owner/skill-b", got)
	}
}

func TestReadSkillsContainer_UsesCorrectFile(t *testing.T) {
	var capturedScript string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			capturedScript = strings.Join(args, " ")
			return "", nil
		},
	}
	ReadSkillsContainer(context.Background(), r)
	if !strings.Contains(capturedScript, FileSkills) {
		t.Errorf("ReadSkillsContainer script = %q, want to reference %s", capturedScript, FileSkills)
	}
}

func TestWriteSkillsContainer_PassesContent(t *testing.T) {
	content := "owner/skill-a\nowner/skill-b"
	var capturedArgs []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			capturedArgs = args
			return "", nil
		},
	}
	if err := WriteSkillsContainer(context.Background(), r, content); err != nil {
		t.Fatalf("WriteSkillsContainer = %v, want nil", err)
	}
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, FileSkills) {
		t.Errorf("WriteSkillsContainer args = %v, want to reference %s", capturedArgs, FileSkills)
	}
	if !strings.Contains(joined, content) {
		t.Errorf("WriteSkillsContainer args = %v, want content %q", capturedArgs, content)
	}
}

func TestReadSkillsFile_Absent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := readSkillsFile()
	if got != "" {
		t.Errorf("readSkillsFile absent = %q, want empty", got)
	}
}

func TestReadSkillsFile_Present(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, FileSkills), []byte("owner/skill-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readSkillsFile()
	if !strings.Contains(got, "owner/skill-a") {
		t.Errorf("readSkillsFile = %q, want to contain owner/skill-a", got)
	}
}
