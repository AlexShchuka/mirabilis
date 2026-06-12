package steps

import (
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func runningContainer(version string) sandbox.Container {
	return sandbox.Container{Running: true, Env: map[string]string{"MIRABILIS_VERSION": version}}
}

func TestImageCheck(t *testing.T) {
	t.Parallel()
	t.Run("fingerprint matches", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect([]string{"git"}, "abc", nil)
		docker := sandbox.NewFakeDocker().StubInspect(runningContainer("abc-"), nil)
		mustCheck(t, &imageStep{d: newTestDeps(t, fake, docker, newFakeStore())}, true)
	})
	t.Run("not running", func(t *testing.T) {
		t.Parallel()
		docker := sandbox.NewFakeDocker().StubInspect(sandbox.Container{}, nil)
		mustCheck(t, &imageStep{d: newTestDeps(t, exec.NewFake(), docker, newFakeStore())}, false)
	})
	t.Run("fingerprint stale", func(t *testing.T) {
		t.Parallel()
		fake := exec.NewFake().Expect([]string{"git"}, "abc", nil)
		docker := sandbox.NewFakeDocker().StubInspect(runningContainer("old-"), nil)
		mustCheck(t, &imageStep{d: newTestDeps(t, fake, docker, newFakeStore())}, false)
	})
}

func TestImageRunForwardsBuildStream(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc", nil).
		Expect([]string{"docker", "compose"}, "#1 building\n#2 done\n", nil)
	s := &imageStep{d: newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore())}
	evs, err := runStep(t, s, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var spawn []string
	var lines []string
	for _, ev := range evs {
		switch ev.Kind {
		case pipeline.EvSpawn:
			spawn = ev.Argv
		case pipeline.EvLine:
			lines = append(lines, ev.Line)
		}
	}
	want := []string{"docker", "compose", "-f", "docker-compose.yml", "build"}
	if !slices.Equal(spawn, want) {
		t.Fatalf("spawn argv = %v, want %v", spawn, want)
	}
	if !slices.Equal(lines, []string{"#1 building", "#2 done"}) {
		t.Fatalf("lines = %v", lines)
	}
}
