package steps

import (
	"errors"
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

func TestSelfUpdateRunsPullThenInstallFromRepo(t *testing.T) {
	fake := exec.NewFake()
	rec := &recordingRunner{inner: fake}
	d := newTestDeps(t, rec, nil, newFakeStore())
	repo := d.Repo

	fake.Expect([]string{"git", "-C", repo, "pull", "--ff-only"}, "Already up to date.", nil)
	fake.Expect([]string{"make", "-C", repo, "install"}, "installed", nil)

	if err := SelfUpdate(t.Context(), d, repo); err != nil {
		t.Fatalf("SelfUpdate = %v, want nil", err)
	}

	specs := rec.Specs()
	if len(specs) != 2 {
		t.Fatalf("recorded %d specs, want 2", len(specs))
	}
	wantPull := []string{"git", "-C", repo, "pull", "--ff-only"}
	if !slices.Equal(specs[0].Argv, wantPull) {
		t.Errorf("first argv = %v, want %v (ff-only pull from repo)", specs[0].Argv, wantPull)
	}
	wantInstall := []string{"make", "-C", repo, "install"}
	if !slices.Equal(specs[1].Argv, wantInstall) {
		t.Errorf("second argv = %v, want %v (make install in repo)", specs[1].Argv, wantInstall)
	}

	snap := d.Obs.Snapshot()
	if st := snap[selfUpdateNode]; st.State != obs.StateOK {
		t.Errorf("selfupdate node state = %v, want OK", st.State)
	}
}

func TestSelfUpdateIdempotentPullIsFastForwardOnly(t *testing.T) {
	fake := exec.NewFake()
	rec := &recordingRunner{inner: fake}
	d := newTestDeps(t, rec, nil, newFakeStore())
	repo := d.Repo

	fake.Expect([]string{"git", "-C", repo, "pull", "--ff-only"}, "Already up to date.", nil)
	fake.Expect([]string{"make", "-C", repo, "install"}, "", nil)

	if err := SelfUpdate(t.Context(), d, repo); err != nil {
		t.Fatalf("SelfUpdate on current repo = %v, want nil (no-op ff-only is idempotent)", err)
	}
	if got := rec.Specs()[0].Argv; !slices.Contains(got, "--ff-only") {
		t.Errorf("pull argv %v missing --ff-only (must never rewrite history)", got)
	}
}

func TestSelfUpdatePullFailureReturnsErrorAndDegrades(t *testing.T) {
	fake := exec.NewFake()
	rec := &recordingRunner{inner: fake}
	d := newTestDeps(t, rec, nil, newFakeStore())
	repo := d.Repo

	fake.Expect([]string{"git", "-C", repo, "pull", "--ff-only"}, "", errors.New("not a fast-forward"))

	err := SelfUpdate(t.Context(), d, repo)
	if err == nil {
		t.Fatal("SelfUpdate = nil on pull failure, want a surfaced error value")
	}

	if len(rec.Specs()) != 1 {
		t.Errorf("recorded %d specs after pull failure, want 1 (install must not run)", len(rec.Specs()))
	}

	snap := d.Obs.Snapshot()
	st, ok := snap[selfUpdateNode]
	if !ok || st.State != obs.StateDegraded {
		t.Errorf("selfupdate node = %+v, want StateDegraded (failure surfaced to obs, not swallowed)", st)
	}
}

func TestUpdateEcosystemRunsContainerProvisionUpdate(t *testing.T) {
	fake := exec.NewFake()
	rec := &recordingRunner{inner: fake}
	d := newTestDeps(t, rec, nil, newFakeStore())

	want := containerArgv("mirabilis", "provision", "--phase", "update")
	fake.Expect(want, "", nil)

	if err := UpdateEcosystem(t.Context(), d); err != nil {
		t.Fatalf("UpdateEcosystem = %v, want nil", err)
	}
	specs := rec.Specs()
	if len(specs) != 1 {
		t.Fatalf("recorded %d specs, want 1", len(specs))
	}
	if !slices.Equal(specs[0].Argv, want) {
		t.Errorf("argv = %v, want %v", specs[0].Argv, want)
	}
}
