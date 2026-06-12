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

func TestContainerSteps_NamesAndMeta(t *testing.T) {
	registered := containerSteps()
	if len(registered) != 2 {
		t.Fatalf("containerSteps() returned %d, want 2", len(registered))
	}
	names := map[string]bool{}
	for _, r := range registered {
		names[r.Meta.Name] = true
	}
	for _, want := range []string{"update", "prepare"} {
		if !names[want] {
			t.Errorf("step %q missing from containerSteps()", want)
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

func TestUpdateRun_FeatureBranch_NoOp(t *testing.T) {
	var cmds []string
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			cmds = append(cmds, name+" "+strings.Join(args, " "))
			if name == "git" && len(args) >= 2 && args[len(args)-1] == "HEAD" {
				return "feature/my-work", nil
			}
			return "", nil
		},
	}
	err := updateStep{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil on feature branch", err)
	}
	for _, c := range cmds {
		if strings.Contains(c, "checkout") || strings.Contains(c, "merge") {
			t.Errorf("Run issued git %q on feature branch, want no checkout/merge", c)
		}
	}
}

func TestUpdateRun_OnMain_FFMerge(t *testing.T) {
	var mergeArgs []string
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 2 && args[len(args)-1] == "HEAD" {
				return "main", nil
			}
			if name == "git" && len(args) >= 2 && args[len(args)-2] == "--ff-only" {
				mergeArgs = args
				return "", nil
			}
			return "", nil
		},
	}
	err := updateStep{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil on main with ff-merge", err)
	}
	if mergeArgs == nil {
		t.Error("git merge --ff-only not issued when on main")
	}
}

func TestUpdateRun_DirtyStatus_Error(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 2 && args[len(args)-1] == "HEAD" {
				return "main", nil
			}
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

func TestUpdateRun_CleanPath_Nil(t *testing.T) {
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			if name == "git" && len(args) >= 2 && args[len(args)-1] == "HEAD" {
				return "main", nil
			}
			return "", nil
		},
	}
	err := updateStep{}.Run(context.Background(), r)
	if err != nil {
		t.Errorf("Run = %v, want nil on clean path", err)
	}
}

func runGitCmd(dir string, args ...string) error {
	out, err := os.StartProcess("/usr/bin/git", append([]string{"git"}, args...), &os.ProcAttr{
		Dir:   dir,
		Files: []*os.File{nil, nil, nil},
	})
	if err != nil {
		return err
	}
	state, err := out.Wait()
	if err != nil {
		return err
	}
	if !state.Success() {
		return fmt.Errorf("git %v: exit %d", args, state.ExitCode())
	}
	return nil
}

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, parts := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
	} {
		if err := runGitCmd(dir, parts[1:]...); err != nil {
			t.Fatalf("git %v: %v", parts[1:], err)
		}
	}
	empty := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, parts := range [][]string{
		{"add", ".gitkeep"},
		{"commit", "-m", "init"},
	} {
		if err := runGitCmd(dir, parts...); err != nil {
			t.Fatalf("git %v: %v", parts, err)
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

func TestPrepareRun_MakeFails_ExactlyOnce(t *testing.T) {
	repo := t.TempDir()
	counter := filepath.Join(t.TempDir(), "make-calls")
	makeDir := makeShim(t, "make", `echo x >> `+counter+`
exit 1`)
	prependPath(t, makeDir)

	r := &runner.FakeRunner{RepoVal: repo}
	err := prepareStep{}.Run(context.Background(), r)
	if err == nil {
		t.Error("prepareStep.Run must error when make fails")
	}
	data, rerr := os.ReadFile(counter)
	if rerr != nil {
		t.Fatalf("counter file not written: %v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("make invoked %d times, want exactly 1", len(lines))
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
