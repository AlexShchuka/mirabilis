package skills

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
		hostSkills    []string
		wantSatisfied bool
	}{
		{
			name:          "both empty",
			containerOut:  "",
			hostSkills:    nil,
			wantSatisfied: true,
		},
		{
			name:          "same single entry",
			containerOut:  "owner/skill-a\n",
			hostSkills:    []string{"owner/skill-a"},
			wantSatisfied: true,
		},
		{
			name:          "same set different order",
			containerOut:  "owner/skill-b\nowner/skill-a\n",
			hostSkills:    []string{"owner/skill-a", "owner/skill-b"},
			wantSatisfied: true,
		},
		{
			name:          "container has more",
			containerOut:  "owner/skill-a\nowner/skill-b\n",
			hostSkills:    []string{"owner/skill-a"},
			wantSatisfied: false,
		},
		{
			name:          "host has more",
			containerOut:  "owner/skill-a\n",
			hostSkills:    []string{"owner/skill-a", "owner/skill-b"},
			wantSatisfied: false,
		},
		{
			name:          "different entries",
			containerOut:  "owner/skill-a\n",
			hostSkills:    []string{"owner/skill-b"},
			wantSatisfied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if len(tt.hostSkills) > 0 {
				csv := strings.Join(tt.hostSkills, ",")
				if err := os.WriteFile(filepath.Join(dir, ".env"),
					[]byte("SKILLS="+csv+"\n"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, ".env"),
		[]byte("SKILLS=owner/skill-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	mskillsFound := false
	provisionFound := false
	for _, c := range calls {
		if strings.Contains(c, "MSKILLS=") {
			mskillsFound = true
		}
		if strings.Contains(c, "mirabilis provision --phase skills") {
			provisionFound = true
		}
	}
	if !mskillsFound {
		t.Errorf("Run success: MSKILLS write call not found; got %v", calls)
	}
	if !provisionFound {
		t.Errorf("Run success: provision --phase skills call not found; got %v", calls)
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
	if meta.Name != "skills" {
		t.Errorf("Steps()[0].Name = %q, want skills", meta.Name)
	}
	if len(meta.Deps) != 1 || meta.Deps[0] != "prepare" {
		t.Errorf("Steps()[0].Deps = %v, want [prepare]", meta.Deps)
	}
	if !meta.Optional {
		t.Error("Steps()[0].Optional should be true")
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
