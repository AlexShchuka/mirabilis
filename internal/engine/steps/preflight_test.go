package steps

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func newPreflightForTest(t *testing.T, fake *exec.Fake) *preflightStep {
	t.Helper()
	s := newPreflight(newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore()))
	s.poll = time.Millisecond
	return s
}

func TestPreflightCheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		stub func(f *exec.Fake)
		want bool
	}{
		{
			name: "docker up and compose valid",
			stub: func(f *exec.Fake) {
				f.Expect([]string{"docker", "version"}, "", nil)
				f.Expect([]string{"docker", "compose"}, "", nil)
			},
			want: true,
		},
		{
			name: "docker down",
			stub: func(f *exec.Fake) {
				f.Expect([]string{"docker", "version"}, "", errors.New("down"))
			},
			want: false,
		},
		{
			name: "compose invalid",
			stub: func(f *exec.Fake) {
				f.Expect([]string{"docker", "version"}, "", nil)
				f.Expect([]string{"docker", "compose"}, "", errors.New("bad yaml"))
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := exec.NewFake()
			tc.stub(fake)
			mustCheck(t, newPreflightForTest(t, fake), tc.want)
		})
	}
}

func TestPreflightRunStartsDockerOnDarwin(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "version"}, "", errors.New("down")).
		Expect([]string{"open", "-a", "Docker"}, "", nil).
		Expect([]string{"docker", "version"}, "", errors.New("starting")).
		Expect([]string{"docker", "version"}, "", nil).
		Expect([]string{"docker", "compose"}, "", nil)
	s := newPreflightForTest(t, fake)
	s.goos = "darwin"
	if _, err := runStep(t, s, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if fake.Remaining() != 0 {
		t.Fatalf("unused stubs: %d", fake.Remaining())
	}
}

func TestPreflightRunOtherGOOSHints(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().Expect([]string{"docker", "version"}, "", errors.New("down"))
	s := newPreflightForTest(t, fake)
	s.goos = "linux"
	_, err := runStep(t, s, nil)
	if err == nil || !strings.Contains(err.Error(), "docker is not running") {
		t.Fatalf("err = %v, want docker-not-running hint", err)
	}
}

func TestPreflightRunPollTimesOut(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "version"}, "", errors.New("down")).
		Expect([]string{"open", "-a", "Docker"}, "", nil)
	for i := 0; i < 3; i++ {
		fake.Expect([]string{"docker", "version"}, "", errors.New("down"))
	}
	s := newPreflightForTest(t, fake)
	s.goos = "darwin"
	s.maxPolls = 3
	_, err := runStep(t, s, nil)
	if err == nil || !strings.Contains(err.Error(), "did not come up") {
		t.Fatalf("err = %v, want did-not-come-up", err)
	}
}

func TestPreflightRunComposeInvalid(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "version"}, "", nil).
		Expect([]string{"docker", "compose"}, "", errors.New("bad yaml"))
	s := newPreflightForTest(t, fake)
	_, err := runStep(t, s, nil)
	if err == nil || !strings.Contains(err.Error(), "compose file invalid") {
		t.Fatalf("err = %v, want compose-invalid", err)
	}
}
