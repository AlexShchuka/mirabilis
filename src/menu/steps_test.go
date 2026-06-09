package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	repo string
	host func(name string, args []string) (string, error)
	cont func(args []string) (string, error)
}

func (f fakeRunner) Repo() string { return f.repo }
func (f fakeRunner) Host(_ context.Context, name string, args ...string) (string, error) {
	if f.host == nil {
		return "", nil
	}
	return f.host(name, args)
}
func (f fakeRunner) Container(_ context.Context, args ...string) (string, error) {
	if f.cont == nil {
		return "", nil
	}
	return f.cont(args)
}

func argsHave(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestCheckUpToDate(t *testing.T) {
	tests := []struct {
		name    string
		giveOut string
		giveErr error
		want    bool
	}{
		{name: "up to date", giveOut: "0", want: true},
		{name: "behind", giveOut: "3", want: false},
		{name: "no upstream", giveErr: errors.New("unknown revision"), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fakeRunner{host: func(_ string, args []string) (string, error) {
				if argsHave(args, "rev-list") {
					return tt.giveOut, tt.giveErr
				}
				return "", nil
			}}
			got, _ := checkUpToDate(context.Background(), r)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadWriteStacksRoundTrip(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("FOO=bar\nSTACKS=old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeStacks(repo, "go,dotnet"); err != nil {
		t.Fatal(err)
	}
	got, ok := readStacks(repo)
	if !ok || got != "go,dotnet" {
		t.Fatalf("readStacks = %q,%v want go,dotnet,true", got, ok)
	}
	data, _ := os.ReadFile(filepath.Join(repo, ".env"))
	s := string(data)
	if !contains(splitLines(s), "FOO=bar") {
		t.Errorf("writeStacks dropped unrelated line FOO=bar: %q", s)
	}
	if count := countLines(s, "STACKS="); count != 1 {
		t.Errorf("writeStacks left %d STACKS= lines, want exactly 1", count)
	}
}

func TestReadStacksAbsent(t *testing.T) {
	if _, ok := readStacks(t.TempDir()); ok {
		t.Error("readStacks reported defined for a repo with no .env")
	}
}

func TestReadStackCatalogSkipsCommentsAndBlanks(t *testing.T) {
	repo := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repo, "config"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "config", "stacks.txt"), []byte("# header\n\ndotnet\n   \nrust\n"), 0o644)
	got := readStackCatalog(repo)
	if want := []string{"dotnet", "rust"}; !reflect.DeepEqual(got, want) {
		t.Errorf("readStackCatalog = %#v, want %#v", got, want)
	}
}

func TestContainerEnvValue(t *testing.T) {
	r := fakeRunner{host: func(_ string, _ []string) (string, error) {
		return "PATH=/usr/bin\nMIRABILIS_VERSION=abc123\nMIRABILIS_STACKS=go,dotnet", nil
	}}
	if got := containerEnvValue(context.Background(), r, "MIRABILIS_VERSION"); got != "abc123" {
		t.Errorf("MIRABILIS_VERSION = %q, want abc123", got)
	}
	if got := containerEnvValue(context.Background(), r, "MISSING"); got != "" {
		t.Errorf("MISSING = %q, want empty", got)
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\n\n b \nc\n")
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("splitLines = %#v, want %#v", got, want)
	}
}

func TestSeedClaudeConfigKeys(t *testing.T) {
	var script string
	r := fakeRunner{cont: func(args []string) (string, error) {
		if len(args) == 3 && args[0] == "bash" && args[1] == "-lc" {
			script = args[2]
		}
		return "", nil
	}}
	if err := seedClaudeConfig(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"hasTrustDialogAccepted", "hasCompletedOnboarding", "bypassPermissionsModeAccepted"} {
		if !strings.Contains(script, key) {
			t.Errorf("seed script is missing %q", key)
		}
	}
}

func TestBuildStepsShape(t *testing.T) {
	byName := map[string]Step{}
	for _, s := range buildSteps() {
		byName[s.Name] = s
	}
	if _, ok := byName["stacks"]; ok {
		t.Error("stacks step should be gone — stacks are configured via the menu")
	}
	if gh, ok := byName["gh"]; !ok || !gh.Interactive {
		t.Error("gh step must exist and be Interactive")
	}
	if _, ok := byName["claude"]; !ok {
		t.Error("claude config step must exist")
	}
	if prepare, ok := byName["prepare"]; !ok || contains(prepare.Deps, "stacks") {
		t.Error("prepare must not depend on a stacks step")
	}
	if h := byName["harness"]; h.Timeout <= 0 {
		t.Error("harness step must have a Timeout to bound the network install")
	}
	if prepare := byName["prepare"]; prepare.Timeout != 0 {
		t.Error("prepare must not be hard-timeout-capped — devcontainer build can be long")
	}
	for _, s := range buildSteps() {
		if s.Detail == "" {
			t.Errorf("step %q has no Detail for the progress hint line", s.Name)
		}
	}
}

func countLines(s, prefix string) int {
	n := 0
	for _, line := range splitLines(s) {
		if len(line) >= len(prefix) && line[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}
