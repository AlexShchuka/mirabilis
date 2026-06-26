package steps

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func TestLaunchContract(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		setup   func(t *testing.T, d Deps, f *exec.Fake, dk *sandbox.FakeDocker, step pipeline.Command)
		resolve func(payload any) pipeline.Result
	}{
		"preflight": {
			setup: func(t *testing.T, _ Deps, f *exec.Fake, _ *sandbox.FakeDocker, step pipeline.Command) {
				p := step.(*preflightStep)
				p.goos = "darwin"
				p.poll = time.Millisecond
				f.Expect([]string{"docker", "version"}, "", errors.New("down")).
					Expect([]string{"docker", "version"}, "", errors.New("down")).
					Expect([]string{"open", "-a", "Docker"}, "", nil).
					Expect([]string{"docker", "version"}, "", nil).
					Expect([]string{"docker", "compose"}, "", nil).
					Expect([]string{"docker", "version"}, "", nil).
					Expect([]string{"docker", "compose"}, "", nil)
			},
		},
		"claude-auth": {},
		"config": {
			resolve: func(any) pipeline.Result {
				return pipeline.Result{Value: WizardResult{Choices: map[string][]string{
					keyStacks:  {"rust"},
					keyPlugins: {},
					keySkills:  {"writer"},
				}}}
			},
		},
		"pull-build": {
			setup: func(_ *testing.T, _ Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				f.Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", errors.New("not found")).
					Expect([]string{"docker", "pull", sandbox.BaseImageBuild}, "", nil).
					Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", nil)
			},
		},
		"pull-runtime": {
			setup: func(_ *testing.T, _ Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				f.Expect([]string{"docker", "image", "inspect", sandbox.BaseImageRuntime}, "", errors.New("not found")).
					Expect([]string{"docker", "pull", sandbox.BaseImageRuntime}, "", nil).
					Expect([]string{"docker", "image", "inspect", sandbox.BaseImageRuntime}, "", nil)
			},
		},
		"image": {
			setup: func(_ *testing.T, _ Deps, f *exec.Fake, dk *sandbox.FakeDocker, _ pipeline.Command) {
				dk.StubInspect(sandbox.Container{}, nil).
					StubInspect(runningContainer("abc-"), nil)
				f.Expect([]string{"git"}, "abc", nil).
					Expect([]string{"docker", "compose"}, "", nil).
					Expect([]string{"git"}, "abc", nil)
			},
		},
		"container": {
			setup: func(t *testing.T, _ Deps, f *exec.Fake, dk *sandbox.FakeDocker, step pipeline.Command) {
				step.(*containerStep).poll = time.Millisecond
				dk.StubInspect(sandbox.Container{}, errors.New("no container")).
					StubInspect(sandbox.Container{}, errors.New("no container")).
					StubInspect(runningContainer("abc-"), nil)
				f.Expect([]string{"git"}, "abc", nil).
					Expect([]string{"docker", "compose"}, "", nil).
					Expect([]string{"git"}, "abc", nil)
			},
		},
		"provision-create": {
			setup: func(_ *testing.T, _ Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				f.Expect([]string{"docker", "exec", "mirabilis", "cat"}, "", errors.New("no such file")).
					Expect([]string{"docker", "exec", "-i", "-e"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "cat"}, "ok\n", nil)
			},
		},
		"provision-start": {
			setup: func(_ *testing.T, _ Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				hash := harness.StartMarkerHash("abc-", testSessionKey)
				f.Expect([]string{"docker", "exec", "mirabilis", "cat"}, "", errors.New("no such file")).
					Expect([]string{"docker", "exec", "-i", "-e"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "cat"}, hash+"\n", nil).
					Expect([]string{"git"}, "abc", nil)
			},
		},
		"gh-auth": {
			setup: func(_ *testing.T, _ Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				f.Expect([]string{"docker", "exec", "mirabilis", "gh", "auth", "status"}, "", errors.New("signed out")).
					Expect([]string{"docker", "exec", "-i", "mirabilis", "env"}, "you are already signed in\n", nil).
					Expect([]string{"docker", "exec", "mirabilis", "gh", "auth", "status"}, "", nil)
			},
		},
		"plugins": {
			setup: func(t *testing.T, d Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				if err := config.WritePluginsDisabled(d.Repo, []string{"alpha"}); err != nil {
					t.Fatal(err)
				}
				f.Expect([]string{"docker", "exec", "mirabilis", "bash"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "env"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "mirabilis"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "bash"}, "alpha\n", nil)
			},
		},
		"skills": {
			setup: func(t *testing.T, d Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				if err := config.WriteSkills(d.Repo, "writer"); err != nil {
					t.Fatal(err)
				}
				f.Expect([]string{"docker", "exec", "mirabilis", "bash"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "env"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "mirabilis"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "bash"}, "writer\n", nil)
			},
		},
		"harness": {
			setup: func(_ *testing.T, _ Deps, f *exec.Fake, _ *sandbox.FakeDocker, _ pipeline.Command) {
				f.Expect(harnessBash(harnessPrefScript), "", nil).
					Expect(harnessBash(harness.ProbeScript), "", errors.New("not installed")).
					Expect(harnessBash("command -v claude"), "/usr/bin/claude", nil).
					Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "marketplace", "add"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "install"}, "", nil).
					Expect([]string{"docker", "exec", "mirabilis", "claude", "plugin", "update"}, "", nil).
					Expect(harnessBash(harness.ProbeScript), "", nil).
					Expect(harnessBash(harness.RelinkScript), "", nil).
					Expect(harnessBash(harnessPrefScript), "", nil).
					Expect(harnessBash(harness.ProbeScript), "", nil)
			},
		},
		"attach": {},
	}
	for _, step := range Launch(newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore())) {
		name := step.Meta().Name
		tc, ok := cases[name]
		if !ok {
			t.Errorf("no contract case for step %q", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := exec.NewFake()
			dk := sandbox.NewFakeDocker()
			d := newTestDeps(t, f, dk, newFakeStore())
			target := findStep(t, Launch(d), name)
			if tc.setup != nil {
				tc.setup(t, d, f, dk, target)
			}
			pipeline.Contract(t, target, tc.resolve)
		})
	}
}
