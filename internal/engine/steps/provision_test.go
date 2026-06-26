package steps

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestProvisionCreateCheck(t *testing.T) {
	t.Parallel()
	cat := []string{"docker", "exec", "mirabilis", "cat", createMarkerPath}
	cases := []struct {
		name string
		out  string
		err  error
		want bool
	}{
		{name: "marker ok", out: "ok\n", want: true},
		{name: "marker missing", err: errors.New("no such file"), want: false},
		{name: "marker not ok", out: "3/14 warned\n", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := exec.NewFake().Expect(cat, tc.out, tc.err)
			s := newProvision(newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore()), phaseCreate)
			mustCheck(t, s, tc.want)
			if got := fake.Calls()[0].Argv; !slices.Equal(got, cat) {
				t.Fatalf("probe argv = %v, want %v", got, cat)
			}
		})
	}
}

func TestProvisionStartCheck(t *testing.T) {
	t.Parallel()
	hash := harness.StartMarkerHash("abc-", testSessionKey)
	cases := []struct {
		name string
		out  string
		err  error
		want bool
	}{
		{name: "hash matches", out: hash + "\n", want: true},
		{name: "hash stale", out: harness.StartMarkerHash("old-", testSessionKey), want: false},
		{name: "marker missing", err: errors.New("no such file"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := exec.NewFake().
				Expect([]string{"docker", "exec", "mirabilis", "cat", startMarkerPath}, tc.out, tc.err)
			if tc.err == nil {
				fake.Expect([]string{"git"}, "abc", nil)
			}
			s := newProvision(newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore()), phaseStart)
			mustCheck(t, s, tc.want)
		})
	}
}

func TestProvisionRunCarriesSelectedLoadout(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{phaseCreate, phaseStart} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			rec := &recordingRunner{inner: exec.NewFake().Expect([]string{"docker", "exec", "-i"}, "", nil)}
			d := newTestDeps(t, rec, sandbox.NewFakeDocker(), newFakeStore())
			if err := config.WriteLoadout(d.Repo, "pvp"); err != nil {
				t.Fatalf("WriteLoadout: %v", err)
			}
			if _, err := runStep(t, newProvision(d, phase), nil); err != nil {
				t.Fatalf("run: %v", err)
			}
			specs := rec.Specs()
			if len(specs) != 1 {
				t.Fatalf("got %d spawns, want 1", len(specs))
			}
			if !slices.Contains(specs[0].Argv, "MIRABILIS_LOADOUT=pvp") {
				t.Fatalf("argv = %v, want it to carry -e MIRABILIS_LOADOUT=pvp", specs[0].Argv)
			}
		})
	}
}

func TestProvisionRunCarriesEffortOverride(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{phaseCreate, phaseStart} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			rec := &recordingRunner{inner: exec.NewFake().Expect([]string{"docker", "exec", "-i"}, "", nil)}
			d := newTestDeps(t, rec, sandbox.NewFakeDocker(), newFakeStore())
			if err := config.WriteEffortOverride(d.Repo, "max"); err != nil {
				t.Fatalf("WriteEffortOverride: %v", err)
			}
			if _, err := runStep(t, newProvision(d, phase), nil); err != nil {
				t.Fatalf("run: %v", err)
			}
			specs := rec.Specs()
			if len(specs) != 1 {
				t.Fatalf("got %d spawns, want 1", len(specs))
			}
			if !slices.Contains(specs[0].Argv, "MIRABILIS_EFFORT=max") {
				t.Fatalf("argv = %v, want it to carry -e MIRABILIS_EFFORT=max", specs[0].Argv)
			}
		})
	}
}

func TestProvisionCheckHonorsDeadline(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().ExpectHang([]string{"docker", "exec", "mirabilis", "cat"})
	s := newProvision(newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore()), phaseCreate)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	ok, err := s.Check(ctx)
	elapsed := time.Since(start)
	if ok {
		t.Fatal("Check = true, want false on hung exec")
	}
	if err != nil {
		t.Fatalf("Check err = %v, want nil", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("Check took %v, want < 3s when parent ctx expires", elapsed)
	}
}

func TestProvisionRunArgvAndStdin(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{phaseCreate, phaseStart} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			rec := &recordingRunner{inner: exec.NewFake().Expect([]string{"docker", "exec", "-i"}, "", nil)}
			d := newTestDeps(t, rec, sandbox.NewFakeDocker(), newFakeStore())
			if _, err := runStep(t, newProvision(d, phase), nil); err != nil {
				t.Fatalf("run: %v", err)
			}
			specs := rec.Specs()
			if len(specs) != 1 {
				t.Fatalf("got %d spawns, want 1", len(specs))
			}
			want := []string{
				"docker", "exec", "-i",
				"-e", "MIRABILIS_LOADOUT=default",
				"-e", "MIRABILIS_EFFORT=",
				"mirabilis",
				"mirabilis", "provision", "--phase", phase, "--proxy-addr", testProxyAddr,
			}
			if !slices.Equal(specs[0].Argv, want) {
				t.Fatalf("argv = %v, want %v", specs[0].Argv, want)
			}
			for _, arg := range specs[0].Argv {
				if strings.Contains(arg, testSessionKey) {
					t.Fatalf("session key leaked into argv: %q", arg)
				}
			}
			stdin, err := io.ReadAll(specs[0].Stdin)
			if err != nil {
				t.Fatal(err)
			}
			if string(stdin) != testSessionKey {
				t.Fatalf("stdin = %q, want session key", stdin)
			}
		})
	}
}
