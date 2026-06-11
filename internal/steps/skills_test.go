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

func TestSkillsCheckSkipsWhenSetsEqual(t *testing.T) {
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

			s := skillsStep{}
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

func TestSkillsRun_Success(t *testing.T) {
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
	impl := skillsStep{}
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

func TestSkillsRun_CallErrors(t *testing.T) {
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
			err := skillsStep{}.Run(context.Background(), r)
			if err == nil {
				t.Errorf("Run %s: expected non-nil error, got nil", tt.name)
			}
		})
	}
}

func TestSkillsSteps_NameAndDeps(t *testing.T) {
	registered := skillsSteps()
	if len(registered) != 1 {
		t.Fatalf("skillsSteps() len = %d, want 1", len(registered))
	}
	meta := registered[0].Meta
	if meta.Name != "skills" {
		t.Errorf("skillsSteps()[0].Name = %q, want skills", meta.Name)
	}
	if len(meta.Deps) != 1 || meta.Deps[0] != "prepare" {
		t.Errorf("skillsSteps()[0].Deps = %v, want [prepare]", meta.Deps)
	}
	if !meta.Optional {
		t.Error("skillsSteps()[0].Optional should be true")
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
