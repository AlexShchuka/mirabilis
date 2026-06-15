package skills

import (
	"context"
	"reflect"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	ghListBoth = `[{"skillName":"golang-naming","sourceURL":"https://github.com/samber/cc-skills-golang"},{"skillName":"golang-testing","sourceURL":"https://github.com/samber/cc-skills-golang"}]`
	ghListOne  = `[{"skillName":"golang-naming","sourceURL":"https://github.com/samber/cc-skills-golang"}]`
)

var golangUnits = []Unit{
	{Repo: "samber/cc-skills-golang", Skill: "golang-naming"},
	{Repo: "samber/cc-skills-golang", Skill: "golang-testing"},
}

func fakeInstaller(f *exec.Fake) Installer {
	return Installer{
		OK: func(ctx context.Context, argv ...string) bool {
			_, err := exec.Run(ctx, f, exec.Spec{Argv: argv})
			return err == nil
		},
		Output: func(ctx context.Context, argv ...string) (string, error) {
			return exec.Run(ctx, f, exec.Spec{Argv: argv})
		},
		Stream: func(ctx context.Context, _ string, _ chan<- pipeline.Event, argv ...string) error {
			for ev := range f.Stream(ctx, exec.Spec{Argv: argv}) {
				if ev.Kind == exec.KindExited {
					return ev.Err
				}
			}
			return nil
		},
	}
}

func apply(t *testing.T, i Installer, units []Unit) error {
	t.Helper()
	out := make(chan pipeline.Event, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range out {
		}
	}()
	err := i.Apply(t.Context(), out, units)
	close(out)
	<-done
	return err
}

func TestUnitsResolvesSelectedGroup(t *testing.T) {
	t.Parallel()
	groups := []config.SkillGroup{
		{Name: "golang", Repo: "samber/cc-skills-golang", Skills: []string{"golang-naming", "golang-testing"}},
		{Name: "python", Repo: "py/repo", Skills: []string{"c"}},
	}
	got := Units(groups, map[string]bool{"golang": true})
	if !reflect.DeepEqual(got, golangUnits) {
		t.Fatalf("Units = %#v, want %#v", got, golangUnits)
	}
}

func TestUnitsNoSelectionNil(t *testing.T) {
	t.Parallel()
	groups := []config.SkillGroup{{Name: "golang", Repo: "r", Skills: []string{"a"}}}
	if got := Units(groups, nil); got != nil {
		t.Fatalf("Units(nil selection) = %#v, want nil", got)
	}
}

func TestUnitsUnselectedGroupExcluded(t *testing.T) {
	t.Parallel()
	groups := []config.SkillGroup{
		{Name: "golang", Repo: "samber/cc-skills-golang", Skills: []string{"golang-naming"}},
		{Name: "python", Repo: "py/repo", Skills: []string{"c"}},
	}
	for _, u := range Units(groups, map[string]bool{"golang": true}) {
		if u.Repo == "py/repo" {
			t.Fatalf("unselected group leaked: %#v", u)
		}
	}
}

func TestSatisfiedAllPresent(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect([]string{"gh", "--version"}, "gh version", nil)
	f.Expect([]string{"gh", "skill", "list"}, ghListBoth, nil)
	ok, err := fakeInstaller(f).Satisfied(t.Context(), golangUnits)
	if err != nil || !ok {
		t.Fatalf("Satisfied = %v, %v; want true, nil", ok, err)
	}
}

func TestSatisfiedFalseWhenMissing(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect([]string{"gh", "--version"}, "gh version", nil)
	f.Expect([]string{"gh", "skill", "list"}, ghListOne, nil)
	ok, _ := fakeInstaller(f).Satisfied(t.Context(), golangUnits)
	if ok {
		t.Fatal("Satisfied true but golang-testing absent")
	}
}

func TestApplyInstallsMissingNoAll(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect([]string{"gh", "--version"}, "gh version", nil)
	f.Expect([]string{"gh", "skill", "list"}, "[]", nil)
	f.Expect([]string{"gh", "skill", "install", "samber/cc-skills-golang", "golang-naming"}, "", nil)
	f.Expect([]string{"gh", "skill", "install", "samber/cc-skills-golang", "golang-testing"}, "", nil)
	if err := apply(t, fakeInstaller(f), golangUnits); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("unused stubs: %d (one install per missing unit)", n)
	}
	for _, c := range f.Calls() {
		for _, a := range c.Argv {
			if a == "--all" {
				t.Fatalf("--all must never appear: %v", c.Argv)
			}
		}
	}
}

func TestApplySkipsInstalled(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	f.Expect([]string{"gh", "--version"}, "gh version", nil)
	f.Expect([]string{"gh", "skill", "list"}, ghListOne, nil)
	f.Expect([]string{"gh", "skill", "install", "samber/cc-skills-golang", "golang-testing"}, "", nil)
	if err := apply(t, fakeInstaller(f), golangUnits); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("expected only the missing unit installed; unused stubs: %d", n)
	}
}

func TestGhMissingGraceful(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	ok, err := fakeInstaller(f).Satisfied(t.Context(), golangUnits)
	if err != nil || !ok {
		t.Fatalf("Satisfied with gh absent = %v, %v; want true, nil", ok, err)
	}
	if err := apply(t, fakeInstaller(f), golangUnits); err != nil {
		t.Fatalf("Apply with gh absent should noop: %v", err)
	}
}

func TestEmptyUnitsNoExec(t *testing.T) {
	t.Parallel()
	f := exec.NewFake()
	ok, _ := fakeInstaller(f).Satisfied(t.Context(), nil)
	if !ok {
		t.Fatal("Satisfied(nil) should be true")
	}
	if err := apply(t, fakeInstaller(f), nil); err != nil {
		t.Fatalf("Apply(nil): %v", err)
	}
	if n := len(f.Calls()); n != 0 {
		t.Errorf("exec ran for empty units: %d calls", n)
	}
}

func TestRepoSlug(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"https://github.com/samber/cc-skills-golang":     "samber/cc-skills-golang",
		"https://github.com/samber/cc-skills-golang.git": "samber/cc-skills-golang",
		"https://github.com/samber/cc-skills-golang/":    "samber/cc-skills-golang",
		"http://github.com/owner/repo":                   "owner/repo",
	}
	for in, want := range cases {
		if got := repoSlug(in); got != want {
			t.Errorf("repoSlug(%q) = %q, want %q", in, got, want)
		}
	}
}
