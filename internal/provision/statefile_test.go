package provision

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestReadContainerFunctions(t *testing.T) {
	tests := []struct {
		name    string
		contOut string
		readFn  func(context.Context, *runner.FakeRunner) string
		wantFn  func(string) bool
		wantMsg string
	}{
		{
			name:    "HarnessChoice skip",
			contOut: "skip\n",
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadHarnessChoiceContainer(ctx, r)
			},
			wantFn:  func(got string) bool { return got == HarnessSkip },
			wantMsg: "want " + HarnessSkip,
		},
		{
			name:    "HarnessChoice install",
			contOut: "install\n",
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadHarnessChoiceContainer(ctx, r)
			},
			wantFn:  func(got string) bool { return got == HarnessInstall },
			wantMsg: "want " + HarnessInstall,
		},
		{
			name:    "DisabledPlugins returns raw",
			contOut: "plugin-x\nplugin-y\n",
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadDisabledPluginsContainer(ctx, r)
			},
			wantFn:  func(got string) bool { return strings.Contains(got, "plugin-x") && strings.Contains(got, "plugin-y") },
			wantMsg: "want plugin-x and plugin-y",
		},
		{
			name:    "Skills returns raw",
			contOut: "owner/skill-a\nowner/skill-b\n",
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadSkillsContainer(ctx, r)
			},
			wantFn: func(got string) bool {
				return strings.Contains(got, "owner/skill-a") && strings.Contains(got, "owner/skill-b")
			},
			wantMsg: "want owner/skill-a and owner/skill-b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contOut := tt.contOut
			r := &runner.FakeRunner{
				ContFunc: func(args []string) (string, error) {
					return contOut, nil
				},
			}
			got := tt.readFn(context.Background(), r)
			if !tt.wantFn(got) {
				t.Errorf("got %q, %s", got, tt.wantMsg)
			}
		})
	}
}

func TestWriteContainerFunctions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		writeFn func(context.Context, *runner.FakeRunner, string) error
		readFn  func(context.Context, *runner.FakeRunner) string
	}{
		{
			name:  "HarnessChoice skip round-trip",
			input: HarnessSkip,
			writeFn: func(ctx context.Context, r *runner.FakeRunner, v string) error {
				return WriteHarnessChoiceContainer(ctx, r, v)
			},
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadHarnessChoiceContainer(ctx, r)
			},
		},
		{
			name:  "HarnessChoice install round-trip",
			input: HarnessInstall,
			writeFn: func(ctx context.Context, r *runner.FakeRunner, v string) error {
				return WriteHarnessChoiceContainer(ctx, r, v)
			},
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadHarnessChoiceContainer(ctx, r)
			},
		},
		{
			name:  "DisabledPlugins round-trip",
			input: "plugin-x\nplugin-y\n",
			writeFn: func(ctx context.Context, r *runner.FakeRunner, v string) error {
				return WriteDisabledPluginsContainer(ctx, r, v)
			},
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadDisabledPluginsContainer(ctx, r)
			},
		},
		{
			name:  "Skills round-trip",
			input: "owner/skill-a\nowner/skill-b\n",
			writeFn: func(ctx context.Context, r *runner.FakeRunner, v string) error {
				return WriteSkillsContainer(ctx, r, v)
			},
			readFn: func(ctx context.Context, r *runner.FakeRunner) string {
				return ReadSkillsContainer(ctx, r)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var captured string
			rw := &runner.FakeRunner{
				ContFunc: func(args []string) (string, error) {
					captured = strings.Join(args, " ")
					return "", nil
				},
			}
			if err := tt.writeFn(ctx, rw, tt.input); err != nil {
				t.Fatalf("write error: %v", err)
			}
			if !strings.Contains(captured, tt.input) {
				t.Errorf("write script does not contain value %q; script: %q", tt.input, captured)
			}
			rr := &runner.FakeRunner{
				ContFunc: func(args []string) (string, error) {
					return tt.input, nil
				},
			}
			got := tt.readFn(ctx, rr)
			want := strings.TrimSpace(tt.input)
			if strings.TrimSpace(got) != want {
				t.Errorf("round-trip %q: got %q, want %q", tt.name, got, want)
			}
		})
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
