package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	cases := []struct {
		name string
		out  string
		err  error
		want bool
	}{
		{"up to date", "0", nil, true},
		{"behind", "3", nil, false},
		{"no upstream", "", errors.New("unknown revision"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := fakeRunner{host: func(_ string, args []string) (string, error) {
				if argsHave(args, "rev-list") {
					return c.out, c.err
				}
				return "", nil
			}}
			got, _ := checkUpToDate(context.Background(), r)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
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

func TestCheckStacks(t *testing.T) {
	defined := t.TempDir()
	_ = os.WriteFile(filepath.Join(defined, ".env"), []byte("STACKS=go\n"), 0o644)
	if ok, _ := checkStacks(context.Background(), fakeRunner{repo: defined}); !ok {
		t.Error("checkStacks should be satisfied when .env defines STACKS")
	}
	if ok, _ := checkStacks(context.Background(), fakeRunner{repo: t.TempDir()}); ok {
		t.Error("checkStacks should not be satisfied when STACKS is unset")
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

func countLines(s, prefix string) int {
	n := 0
	for _, line := range splitLines(s) {
		if len(line) >= len(prefix) && line[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}
