package steps

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func newContainerForTest(t *testing.T, fake *exec.Fake, docker *sandbox.FakeDocker) *containerStep {
	t.Helper()
	s := newContainer(newTestDeps(t, fake, docker, newFakeStore()))
	s.poll = time.Millisecond
	s.wait = 50 * time.Millisecond
	return s
}

func TestContainerCheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		c    sandbox.Container
		err  error
		git  bool
		want bool
	}{
		{name: "healthy match", c: runningContainer("abc-"), git: true, want: true},
		{name: "inspect error", err: errors.New("no container"), want: false},
		{name: "not running", c: sandbox.Container{}, want: false},
		{
			name: "unhealthy",
			c: sandbox.Container{
				Running: true,
				Health:  "unhealthy",
				Env:     map[string]string{"MIRABILIS_VERSION": "abc-"},
			},
			want: false,
		},
		{name: "version mismatch", c: runningContainer("old-"), git: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := exec.NewFake()
			if tc.git {
				fake.Expect([]string{"git"}, "abc", nil)
			}
			docker := sandbox.NewFakeDocker().StubInspect(tc.c, tc.err)
			mustCheck(t, newContainerForTest(t, fake, docker), tc.want)
		})
	}
}

func TestContainerRunUpAndWaitHealthy(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc", nil).
		Expect([]string{"docker", "compose", "-f", "docker-compose.yml", "up", "-d"}, "", nil)
	docker := sandbox.NewFakeDocker().
		StubInspect(sandbox.Container{}, errors.New("no container")).
		StubInspect(sandbox.Container{Running: true, Health: "starting"}, nil).
		StubInspect(sandbox.Container{Running: true, Health: "healthy"}, nil)
	s := newContainerForTest(t, fake, docker)
	if _, err := runStep(t, s, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if fake.Remaining() != 0 {
		t.Fatalf("unused stubs: %d", fake.Remaining())
	}
}

func TestContainerRunRecreatesStale(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc", nil).
		Expect([]string{"git"}, "abc", nil).
		Expect([]string{"git"}, "abc", nil).
		Expect([]string{"docker", "compose", "-f", "docker-compose.yml", "down"}, "", nil).
		Expect([]string{"docker", "compose", "-f", "docker-compose.yml", "up", "-d"}, "", nil)
	docker := sandbox.NewFakeDocker().
		StubInspect(runningContainer("old-"), nil).
		StubInspect(runningContainer("abc-"), nil)
	s := newContainerForTest(t, fake, docker)
	evs, err := runStep(t, s, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var spawns [][]string
	for _, ev := range evs {
		if ev.Kind == pipeline.EvSpawn {
			spawns = append(spawns, ev.Argv)
		}
	}
	if len(spawns) != 2 || spawns[0][4] != "down" || spawns[1][4] != "up" {
		t.Fatalf("spawns = %v, want down then up", spawns)
	}
}

func TestContainerRunHealthTimeout(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc", nil).
		Expect([]string{"docker", "compose"}, "", nil)
	docker := sandbox.NewFakeDocker().
		StubInspect(sandbox.Container{}, errors.New("no container")).
		StubInspect(sandbox.Container{Running: true, Health: "starting"}, nil)
	s := newContainerForTest(t, fake, docker)
	s.wait = 5 * time.Millisecond
	_, err := runStep(t, s, nil)
	if err == nil || !strings.Contains(err.Error(), "did not become healthy") {
		t.Fatalf("err = %v, want health timeout", err)
	}
}
