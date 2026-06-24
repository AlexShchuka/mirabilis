package provision

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func ecosystemTestDir(d Deps, name string) string {
	return filepath.Join(d.Home, ecosystemDirRel, name)
}

func TestEcosystemCheckSkipsWhenGHUnauthed(t *testing.T) {
	d, f := testDeps(t)
	f.Expect([]string{"gh", "auth", "status"}, "", errors.New("not logged in"))
	step := &ecosystemStep{d: d}
	if !checkStep(t, step) {
		t.Error("check must be true (skip) when gh is not authenticated")
	}
}

func TestEcosystemCheckFalseWhenAnyRepoMissing(t *testing.T) {
	d, f := testDeps(t)
	f.Expect([]string{"gh", "auth", "status"}, "", nil)
	for range ecosystemRepos {
		f.Expect([]string{"bash", "-lc"}, "", errors.New("absent"))
	}
	step := &ecosystemStep{d: d}
	if checkStep(t, step) {
		t.Error("check must be false when a repo is not cloned")
	}
}

func TestEcosystemRunClonesMissingPullsPresent(t *testing.T) {
	d, f := testDeps(t)

	present := ecosystemRepos[1]
	mustWrite(t, filepath.Join(ecosystemTestDir(d, present), ".git", "HEAD"), "ref: refs/heads/main\n")

	f.Expect([]string{"gh", "auth", "status"}, "", nil)
	f.Expect(script("gh auth setup-git"), "", nil)
	f.Expect(script(fmt.Sprintf(`mkdir -p %q`, filepath.Join(d.Home, ecosystemDirRel))), "", nil)
	for _, name := range ecosystemRepos {
		dir := ecosystemTestDir(d, name)
		probe := fmt.Sprintf(`test -d %q`, filepath.Join(dir, ".git"))
		if name == present {
			f.Expect(script(probe), "", nil)
			f.Expect(script(fmt.Sprintf(`git -C %q pull --ff-only`, dir)), "", nil)
			continue
		}
		f.Expect(script(probe), "", errors.New("absent"))
		url := fmt.Sprintf("https://github.com/AlexShchuka/%s.git", name)
		f.Expect(script(fmt.Sprintf(`git clone %q %q`, url, dir)), "", nil)
	}

	step := &ecosystemStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("run left %d unused stubs", n)
	}
}

func TestEcosystemRunSkipsWhenGHUnauthed(t *testing.T) {
	d, f := testDeps(t)
	f.Expect([]string{"gh", "auth", "status"}, "", errors.New("not logged in"))
	step := &ecosystemStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("run made %d calls, want 1 (gh auth status only): %v", len(calls), calls)
	}
}

func TestEcosystemRegisteredInCarryStart(t *testing.T) {
	d, _ := testDeps(t)
	steps := carryStart(d)
	found := false
	for _, s := range steps {
		if s.Meta().Name == "ecosystem" {
			found = true
		}
	}
	if !found {
		t.Error("ecosystem step not registered in carryStart")
	}
}
