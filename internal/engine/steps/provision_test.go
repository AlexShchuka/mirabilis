package steps

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestStartMarkerHash(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("fp-1" + "key-1"))
	if got := startMarkerHash("fp-1", "key-1"); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("hash = %q", got)
	}
}

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
	hash := startMarkerHash("abc-", testSessionKey)
	cases := []struct {
		name string
		out  string
		err  error
		want bool
	}{
		{name: "hash matches", out: hash + "\n", want: true},
		{name: "hash stale", out: startMarkerHash("old-", testSessionKey), want: false},
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
				"docker", "exec", "-i", "mirabilis",
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
