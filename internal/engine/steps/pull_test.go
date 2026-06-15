package steps

import (
	"errors"
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestPullBuildMeta(t *testing.T) {
	t.Parallel()
	s := newPullBuild(newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore()))
	m := s.Meta()
	if m.Name != "pull-build" {
		t.Errorf("name = %q, want pull-build", m.Name)
	}
	if !m.Parallel {
		t.Error("Parallel = false, want true")
	}
	if m.Kind != pipeline.Auto {
		t.Errorf("Kind = %v, want Auto", m.Kind)
	}
	if !slices.Equal(m.Deps, []string{"preflight"}) {
		t.Errorf("Deps = %v, want [preflight]", m.Deps)
	}
}

func TestPullRuntimeMeta(t *testing.T) {
	t.Parallel()
	s := newPullRuntime(newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore()))
	m := s.Meta()
	if m.Name != "pull-runtime" {
		t.Errorf("name = %q, want pull-runtime", m.Name)
	}
	if !m.Parallel {
		t.Error("Parallel = false, want true")
	}
}

func TestPullCheckPresent(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", nil)
	s := newPullBuild(newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore()))
	mustCheck(t, s, true)
}

func TestPullCheckMissing(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", errors.New("not found"))
	s := newPullBuild(newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore()))
	mustCheck(t, s, false)
}

func TestPullRunForwardsStream(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "pull", sandbox.BaseImageBuild}, "Pulling from library/golang\n", nil)
	s := newPullBuild(newTestDeps(t, fake, sandbox.NewFakeDocker(), newFakeStore()))
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
	want := []string{"docker", "pull", sandbox.BaseImageBuild}
	if !slices.Equal(spawn, want) {
		t.Fatalf("spawn argv = %v, want %v", spawn, want)
	}
	if !slices.Equal(lines, []string{"Pulling from library/golang"}) {
		t.Fatalf("lines = %v", lines)
	}
}
