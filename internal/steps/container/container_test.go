package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestSteps_NamesAndMeta(t *testing.T) {
	registered := Steps()
	if len(registered) != 2 {
		t.Fatalf("Steps() returned %d, want 2", len(registered))
	}
	names := map[string]bool{}
	for _, r := range registered {
		names[r.Meta.Name] = true
	}
	for _, want := range []string{"update", "prepare"} {
		if !names[want] {
			t.Errorf("step %q missing from Steps()", want)
		}
	}
}

func TestUpdateCheck_RevListZero_Satisfied(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "-C" {
				if len(args) >= 3 && args[2] == "fetch" {
					return "", nil
				}
				if len(args) >= 3 && args[2] == "rev-list" {
					return "0", nil
				}
			}
			return "", nil
		},
	}
	got, err := updateStep{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got {
		t.Error("Check = false, want true when rev-list returns 0")
	}
}

func TestUpdateCheck_RevListNonZero_Unsatisfied(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "-C" {
				if len(args) >= 3 && args[2] == "fetch" {
					return "", nil
				}
				if len(args) >= 3 && args[2] == "rev-list" {
					return "3", nil
				}
			}
			return "", nil
		},
	}
	got, err := updateStep{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got {
		t.Error("Check = true, want false when rev-list returns 3 (3 commits behind)")
	}
}

func TestUpdateCheck_RevListError_Satisfied(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) > 0 && args[0] == "-C" {
				if len(args) >= 3 && args[2] == "fetch" {
					return "", nil
				}
				if len(args) >= 3 && args[2] == "rev-list" {
					return "", fmt.Errorf("not a git repo")
				}
			}
			return "", nil
		},
	}
	got, err := updateStep{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got {
		t.Error("Check = false, want true when rev-list errors (best-effort)")
	}
}

func TestUpdateRun_DirtyStatus_Error(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 3 && args[2] == "status" {
				return "M internal/foo.go", nil
			}
			return "", nil
		},
	}
	err := updateStep{}.Run(context.Background(), r)
	if err == nil {
		t.Error("Run must error when working tree is dirty")
	}
}

func TestUpdateRun_CheckoutError_Propagated(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 3 && args[2] == "status" {
				return "", nil
			}
			if name == "git" && len(args) >= 3 && args[2] == "checkout" {
				return "", fmt.Errorf("checkout failed")
			}
			return "", nil
		},
	}
	err := updateStep{}.Run(context.Background(), r)
	if err == nil {
		t.Error("Run must propagate checkout error")
	}
}

func TestUpdateRun_CleanPath_Nil(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "", nil
		},
	}
	err := updateStep{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil on clean path", err)
	}
}

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, cmd := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "t@example.com"},
		{"git", "config", "user.name", "T"},
	} {
		proc, err := os.StartProcess("/usr/bin/git", append([]string{"git"}, cmd[1:]...), &os.ProcAttr{
			Dir:   dir,
			Files: []*os.File{nil, nil, nil},
		})
		if err != nil {
			t.Fatalf("StartProcess %v: %v", cmd, err)
		}
		if st, err := proc.Wait(); err != nil || !st.Success() {
			t.Fatalf("git %v: %v", cmd[1:], err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", ".gitkeep"}, {"commit", "-m", "init"}} {
		proc, err := os.StartProcess("/usr/bin/git", append([]string{"git"}, args...), &os.ProcAttr{
			Dir:   dir,
			Files: []*os.File{nil, nil, nil},
		})
		if err != nil {
			t.Fatal(err)
		}
		if st, err := proc.Wait(); err != nil || !st.Success() {
			t.Fatalf("git %v failed", args)
		}
	}
	return dir
}

func TestPrepareCheck_RunningNotStale_Satisfied(t *testing.T) {
	repo := makeGitRepo(t)
	sha := gitShort(t, repo)

	r := &runner.FakeRunner{
		RepoVal: repo,
		HostFunc: func(name string, args []string) (string, error) {
			if name == "docker" {
				if len(args) > 0 && args[0] == "container" {
					return "true", nil
				}
				return "MIRABILIS_VERSION=" + sha + "\nMIRABILIS_STACKS=\n", nil
			}
			if name == "git" {
				return sha, nil
			}
			return "", nil
		},
	}
	got, err := prepareStep{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got {
		t.Error("Check = false when container running and not stale")
	}
}

func TestPrepareCheck_NotRunning_Unsatisfied(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "false", nil
		},
	}
	got, err := prepareStep{}.Check(context.Background(), r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got {
		t.Error("Check = true when container not running, want false")
	}
}

func TestPrepareRun_MakeFails(t *testing.T) {
	repo := t.TempDir()
	makeDir := makeShim(t, "make", `echo "make failed"; exit 1`)
	prependPath(t, makeDir)

	r := &runner.FakeRunner{RepoVal: repo}
	err := prepareStep{}.Run(context.Background(), r)
	if err == nil {
		t.Error("prepareStep.Run must error when make fails")
	}
}

func TestPrepareRun_DevcontainerFails(t *testing.T) {
	repo := t.TempDir()
	makeDir := makeShim(t, "make", `exit 0`)
	devcontainerDir := makeShim(t, "devcontainer", `echo "up failed"; exit 1`)
	prependPath(t, makeDir, devcontainerDir)

	r := &runner.FakeRunner{RepoVal: repo}
	err := prepareStep{}.Run(context.Background(), r)
	if err == nil {
		t.Error("prepareStep.Run must error when devcontainer up fails")
	}
}

func TestPrepareRun_Success(t *testing.T) {
	repo := t.TempDir()
	makeDir := makeShim(t, "make", `exit 0`)
	devcontainerDir := makeShim(t, "devcontainer", `exit 0`)
	prependPath(t, makeDir, devcontainerDir)

	r := &runner.FakeRunner{RepoVal: repo}
	err := prepareStep{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("prepareStep.Run = %v, want nil on success", err)
	}
}

func makeShim(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func prependPath(t *testing.T, dirs ...string) {
	t.Helper()
	base := os.Getenv("PATH")
	prefix := ""
	for _, d := range dirs {
		if prefix != "" {
			prefix += ":"
		}
		prefix += d
	}
	t.Setenv("PATH", prefix+":"+base)
}

func gitShort(t *testing.T, dir string) string {
	t.Helper()
	out, err := os.ReadFile(filepath.Join(dir, ".git", "refs", "heads", "master"))
	if err != nil {
		out, err = os.ReadFile(filepath.Join(dir, ".git", "refs", "heads", "main"))
	}
	if err != nil {
		t.Fatalf("gitShort: cannot find HEAD ref: %v", err)
	}
	sha := string(out)
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
