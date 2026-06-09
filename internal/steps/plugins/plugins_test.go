package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestCheckSkipsWhenSetsEqual(t *testing.T) {
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

			s := step{}
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

func TestRun_Success(t *testing.T) {
	dir := t.TempDir()
	var calls []string
	r := &runner.FakeRunner{
		RepoVal: dir,
		ContFunc: func(args []string) (string, error) {
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}
	impl := step{}
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

func TestRun_FirstCallError(t *testing.T) {
	dir := t.TempDir()
	callCount := 0
	r := &runner.FakeRunner{
		RepoVal: dir,
		ContFunc: func(args []string) (string, error) {
			callCount++
			if callCount == 1 {
				return "", fmt.Errorf("env write failed")
			}
			return "", nil
		},
	}
	impl := step{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Error("Run first-call error: expected non-nil error, got nil")
	}
	if callCount != 1 {
		t.Errorf("Run first-call error: expected exactly 1 call before return, got %d", callCount)
	}
}

func TestRun_SecondCallError(t *testing.T) {
	dir := t.TempDir()
	callCount := 0
	r := &runner.FakeRunner{
		RepoVal: dir,
		ContFunc: func(args []string) (string, error) {
			callCount++
			if callCount == 2 {
				return "", fmt.Errorf("provision failed")
			}
			return "", nil
		},
	}
	impl := step{}
	err := impl.Run(context.Background(), r)
	if err == nil {
		t.Error("Run second-call error: expected non-nil error, got nil")
	}
}

func TestSteps_NameAndDeps(t *testing.T) {
	registered := Steps()
	if len(registered) != 1 {
		t.Fatalf("Steps() len = %d, want 1", len(registered))
	}
	meta := registered[0].Meta
	if meta.Name != "plugins" {
		t.Errorf("Steps()[0].Name = %q, want plugins", meta.Name)
	}
	if len(meta.Deps) != 1 || meta.Deps[0] != "prepare" {
		t.Errorf("Steps()[0].Deps = %v, want [prepare]", meta.Deps)
	}
}

func TestSetsEqual(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{}, nil, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a"}, []string{"b"}, false},
		{[]string{"a", "b"}, []string{"a"}, false},
	}
	for _, tt := range tests {
		if got := setsEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("setsEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
