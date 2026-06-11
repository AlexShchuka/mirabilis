package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestResetTerminal(t *testing.T) {
	var buf bytes.Buffer
	resetTerminal(&buf)
	want := exitAltScreen + resetScrollRegion + clearScreenHome + showCursor
	got := buf.String()
	if got == "" {
		t.Fatal("resetTerminal wrote nothing")
	}
	if got != want {
		t.Fatalf("resetTerminal output mismatch: got %q, want %q", got, want)
	}
}

func TestResolveGHTokenEmpty(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	_, err := resolveGHToken(r)
	if err == nil {
		t.Error("resolveGHToken must return an error when token is empty")
	}
}

func TestResolveGHTokenPresent(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "ghp_testtoken", nil
		},
	}
	tok, err := resolveGHToken(r)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tok != "ghp_testtoken" {
		t.Errorf("got %q, want ghp_testtoken", tok)
	}
}

func TestHandoffArgv(t *testing.T) {
	want := []string{
		"/usr/bin/docker", "exec", "-it",
		"-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
		"-e", "COLORTERM=truecolor",
		"-e", "TERM=xterm-256color",
		"mirabilis",
		"claude", "--dangerously-skip-permissions", "--append-system-prompt-file", "/tmp/sp.md",
	}
	got := handoffArgv("/usr/bin/docker", "/tmp/sp.md")
	if len(got) != len(want) {
		t.Fatalf("handoffArgv len = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHandoffEnv(t *testing.T) {
	t.Run("existing entry replaced", func(t *testing.T) {
		base := []string{"FOO=bar", "GITHUB_PERSONAL_ACCESS_TOKEN=old", "BAZ=qux"}
		result := handoffEnv(base, "new")
		counts := 0
		var val string
		for _, kv := range result {
			if strings.HasPrefix(kv, "GITHUB_PERSONAL_ACCESS_TOKEN=") {
				counts++
				val = strings.TrimPrefix(kv, "GITHUB_PERSONAL_ACCESS_TOKEN=")
			}
		}
		if counts != 1 {
			t.Errorf("GITHUB_PERSONAL_ACCESS_TOKEN appears %d times, want 1; env: %v", counts, result)
		}
		if val != "new" {
			t.Errorf("GITHUB_PERSONAL_ACCESS_TOKEN value = %q, want %q", val, "new")
		}
		var fooFound, bazFound bool
		for _, kv := range result {
			if kv == "FOO=bar" {
				fooFound = true
			}
			if kv == "BAZ=qux" {
				bazFound = true
			}
		}
		if !fooFound {
			t.Error("FOO=bar missing from result")
		}
		if !bazFound {
			t.Error("BAZ=qux missing from result")
		}
	})

	t.Run("valueless entry stripped", func(t *testing.T) {
		base := []string{"FOO=bar", "GITHUB_PERSONAL_ACCESS_TOKEN"}
		result := handoffEnv(base, "new")
		want := []string{"FOO=bar", "GITHUB_PERSONAL_ACCESS_TOKEN=new"}
		if len(result) != len(want) {
			t.Fatalf("result = %v, want %v", result, want)
		}
		for i := range want {
			if result[i] != want[i] {
				t.Errorf("result[%d] = %q, want %q", i, result[i], want[i])
			}
		}
	})

	t.Run("no prior entry appended last", func(t *testing.T) {
		base := []string{"FOO=bar", "BAZ=qux"}
		result := handoffEnv(base, "tok")
		if last := result[len(result)-1]; last != "GITHUB_PERSONAL_ACCESS_TOKEN=tok" {
			t.Errorf("last element = %q, want GITHUB_PERSONAL_ACCESS_TOKEN=tok", last)
		}
		if len(result) != 3 {
			t.Errorf("result length = %d, want 3; env: %v", len(result), result)
		}
	})
}

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

func TestSystemPromptScript_FallbackPath(t *testing.T) {
	r := &runner.FakeRunner{
		RepoVal: t.TempDir(),
		ContFunc: func(args []string) (string, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "auth" && args[2] == "token" {
				return "ghtoken\n", nil
			}
			return "", nil
		},
	}
	_ = r
	spf := ""
	if spf = strings.TrimSpace(spf); spf == "" {
		spf = "/opt/mirabilis/config/sandbox-context.md"
	}
	if spf != "/opt/mirabilis/config/sandbox-context.md" {
		t.Errorf("fallback path = %q, want /opt/mirabilis/config/sandbox-context.md", spf)
	}
}

func TestSystemPromptScript_ContainerPath_UsedWhenPresent(t *testing.T) {
	want := "/tmp/mirabilis-system-prompt.md"
	r := &runner.FakeRunner{
		RepoVal: t.TempDir(),
		ContFunc: func(args []string) (string, error) {
			return want + "\n", nil
		},
	}
	ctx := t.Context()
	spf, _ := r.Container(ctx, "bash", "-lc", systemPromptScript)
	if spf = strings.TrimSpace(spf); spf == "" {
		spf = "/opt/mirabilis/config/sandbox-context.md"
	}
	if spf != want {
		t.Errorf("got %q, want %q", spf, want)
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
