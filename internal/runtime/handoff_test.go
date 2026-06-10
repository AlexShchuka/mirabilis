package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestHandoff_NoGHToken_Errors(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "auth" && args[2] == "token" {
				return "", fmt.Errorf("not signed in")
			}
			return "", nil
		},
	}
	err := Handoff(r)
	if err == nil {
		t.Fatal("Handoff = nil, want error when no GitHub token is available")
	}
	if !strings.Contains(err.Error(), "GitHub token is not available") {
		t.Errorf("err = %v, want GitHub-token-unavailable message", err)
	}
}

func TestHandoff_DockerMissing_Errors(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r := &runner.FakeRunner{
		RepoVal: t.TempDir(),
		ContFunc: func(args []string) (string, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "auth" && args[2] == "token" {
				return "ghtoken\n", nil
			}
			return "", nil
		},
	}
	err := Handoff(r)
	if err == nil {
		t.Fatal("Handoff = nil, want error when docker is not on PATH")
	}
}
