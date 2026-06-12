package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestPluginsCheckSkipsWhenSetsEqual(t *testing.T) {
	tests := []struct {
		name          string
		containerOut  string
		hostDisabled  []string
		wantSatisfied bool
	}{
		{
			name:          "both empty",
			containerOut:  "",
			hostDisabled:  nil,
			wantSatisfied: true,
		},
		{
			name:          "same single entry",
			containerOut:  "plugin-a\n",
			hostDisabled:  []string{"plugin-a"},
			wantSatisfied: true,
		},
		{
			name:          "same set different order",
			containerOut:  "plugin-b\nplugin-a\n",
			hostDisabled:  []string{"plugin-a", "plugin-b"},
			wantSatisfied: true,
		},
		{
			name:          "container has more",
			containerOut:  "plugin-a\nplugin-b\n",
			hostDisabled:  []string{"plugin-a"},
			wantSatisfied: false,
		},
		{
			name:          "host has more",
			containerOut:  "plugin-a\n",
			hostDisabled:  []string{"plugin-a", "plugin-b"},
			wantSatisfied: false,
		},
		{
			name:          "different entries",
			containerOut:  "plugin-a\n",
			hostDisabled:  []string{"plugin-b"},
			wantSatisfied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if len(tt.hostDisabled) > 0 {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				csv := ""
				for i, p := range tt.hostDisabled {
					if i > 0 {
						csv += ","
					}
					csv += p
				}
				if err := os.WriteFile(filepath.Join(dir, ".env"),
					[]byte("PLUGINS_DISABLED="+csv+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			r := &runner.FakeRunner{
				RepoVal: dir,
				ContFunc: func(args []string) (string, error) {
					return tt.containerOut, nil
				},
			}

			s := pluginsStep{}
			got, err := s.Check(context.Background(), r)
			if err != nil {
				t.Fatalf("Check returned error: %v", err)
			}
			if got != tt.wantSatisfied {
				t.Errorf("Check = %v, want %v", got, tt.wantSatisfied)
			}
		})
	}
}

func TestPluginsRun_Success(t *testing.T) {
	dir := t.TempDir()
	var calls []string
	r := &runner.FakeRunner{
		RepoVal: dir,
		ContFunc: func(args []string) (string, error) {
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}
	impl := pluginsStep{}
	if err := impl.Run(context.Background(), r); err != nil {
		t.Errorf("Run success = %v, want nil", err)
	}
	if len(calls) < 2 {
		t.Fatalf("Run success: expected at least 2 container calls, got %d: %v", len(calls), calls)
	}
	mdisFound := false
	provisionFound := false
	for _, c := range calls {
		if strings.Contains(c, "MDIS=") {
			mdisFound = true
		}
		if strings.Contains(c, "mirabilis provision --phase plugins") {
			provisionFound = true
		}
	}
	if !mdisFound {
		t.Errorf("Run success: MDIS write call not found; got %v", calls)
	}
	if !provisionFound {
		t.Errorf("Run success: provision --phase plugins call not found; got %v", calls)
	}
}

func TestPluginsRun_CallErrors(t *testing.T) {
	tests := []struct {
		name       string
		failOnCall int
	}{
		{"first call error", 1},
		{"second call error", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			callCount := 0
			r := &runner.FakeRunner{
				RepoVal: dir,
				ContFunc: func(args []string) (string, error) {
					callCount++
					if callCount == tt.failOnCall {
						return "", fmt.Errorf("call %d failed", tt.failOnCall)
					}
					return "", nil
				},
			}
			err := pluginsStep{}.Run(context.Background(), r)
			if err == nil {
				t.Errorf("Run %s: expected non-nil error, got nil", tt.name)
			}
		})
	}
}

func TestPluginsSteps_NameAndDeps(t *testing.T) {
	registered := pluginsSteps()
	if len(registered) != 1 {
		t.Fatalf("pluginsSteps() len = %d, want 1", len(registered))
	}
	meta := registered[0].Meta
	if meta.Name != "plugins" {
		t.Errorf("pluginsSteps()[0].Name = %q, want plugins", meta.Name)
	}
	if len(meta.Deps) != 1 || meta.Deps[0] != "prepare" {
		t.Errorf("pluginsSteps()[0].Deps = %v, want [prepare]", meta.Deps)
	}
}
