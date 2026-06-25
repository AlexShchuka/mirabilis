package steps

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

var errPullMissing = errors.New("not found")

type satisfiedStep struct {
	name string
	deps []string
}

func (s *satisfiedStep) Meta() pipeline.Meta {
	return pipeline.Meta{Name: s.name, Title: s.name, Deps: s.deps, Kind: pipeline.Auto}
}

func (s *satisfiedStep) Check(context.Context) (bool, error) { return true, nil }

func (s *satisfiedStep) Run(context.Context, chan<- pipeline.Event, <-chan pipeline.Result) error {
	return nil
}

type barrierRunner struct {
	inner   exec.Runner
	arrived chan struct{}
	proceed chan struct{}

	mu   sync.Mutex
	once sync.Once
}

func newBarrierRunner(inner exec.Runner, parties int) *barrierRunner {
	return &barrierRunner{
		inner:   inner,
		arrived: make(chan struct{}, parties),
		proceed: make(chan struct{}),
	}
}

func (r *barrierRunner) Stream(ctx context.Context, spec exec.Spec) <-chan exec.Event {
	isPull := len(spec.Argv) >= 2 && spec.Argv[0] == "docker" && spec.Argv[1] == "pull"
	if !isPull {
		return r.inner.Stream(ctx, spec)
	}
	r.arrived <- struct{}{}
	if len(r.arrived) == cap(r.arrived) {
		r.once.Do(func() { close(r.proceed) })
	}
	select {
	case <-r.proceed:
	case <-ctx.Done():
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inner.Stream(ctx, spec)
}

func TestPullStepsRunConcurrentlyInDefaultLaunch(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", errPullMissing).
		Expect([]string{"docker", "pull", sandbox.BaseImageBuild}, "done\n", nil).
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", nil).
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageRuntime}, "", errPullMissing).
		Expect([]string{"docker", "pull", sandbox.BaseImageRuntime}, "done\n", nil).
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageRuntime}, "", nil)
	runner := newBarrierRunner(fake, 2)
	d := newTestDeps(t, runner, sandbox.NewFakeDocker(), newFakeStore())

	p, err := pipeline.New(nil, &satisfiedStep{name: "preflight"}, newPullBuild(d), newPullRuntime(d))
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := runPipelineSteps(t, ctx, p); err != nil {
		t.Fatalf("run: %v", err)
	}
	select {
	case <-runner.proceed:
	default:
		t.Fatal("both pull steps never overlapped at the docker-pull barrier (IO steps did not run concurrently)")
	}
}

func TestComputeImageWaitsForBothPullsInDefaultLaunch(t *testing.T) {
	t.Parallel()
	fake := exec.NewFake().
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", errPullMissing).
		Expect([]string{"docker", "pull", sandbox.BaseImageBuild}, "done\n", nil).
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageBuild}, "", nil).
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageRuntime}, "", errPullMissing).
		Expect([]string{"docker", "pull", sandbox.BaseImageRuntime}, "done\n", nil).
		Expect([]string{"docker", "image", "inspect", sandbox.BaseImageRuntime}, "", nil).
		Expect([]string{"git"}, "abc", nil).
		Expect([]string{"docker", "compose"}, "Building\n", nil)
	rec := &recordingRunner{inner: fake}
	dk := sandbox.NewFakeDocker().StubInspect(sandbox.Container{}, nil)
	d := newTestDeps(t, rec, dk, newFakeStore())

	p, err := pipeline.New(nil,
		&satisfiedStep{name: "preflight"},
		&satisfiedStep{name: "config"},
		newPullBuild(d), newPullRuntime(d), &imageStep{d: d},
	)
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := runPipelineSteps(t, ctx, p); err != nil {
		t.Fatalf("run: %v", err)
	}

	pulls, build := pullsBeforeBuild(rec.Specs())
	if !build {
		t.Fatal("compose build never ran")
	}
	if pulls != 2 {
		t.Fatalf("docker pulls before build = %d, want 2 (image compute must wait for both IO pulls)", pulls)
	}
}

func pullsBeforeBuild(specs []exec.Spec) (int, bool) {
	pulls := 0
	for _, spec := range specs {
		if len(spec.Argv) < 2 || spec.Argv[0] != "docker" {
			continue
		}
		switch spec.Argv[1] {
		case "pull":
			pulls++
		case "compose":
			return pulls, true
		}
	}
	return pulls, false
}

func TestOnlyIndependentIOIsParallelInLaunch(t *testing.T) {
	t.Parallel()
	d := newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore())

	var parallel []string
	for _, s := range Launch(d) {
		if m := s.Meta(); m.Parallel {
			parallel = append(parallel, m.Name)
		}
	}

	wantParallel := map[string]bool{"pull-build": true, "pull-runtime": true}
	for _, name := range parallel {
		if !wantParallel[name] {
			t.Errorf("step %q is Parallel; only the independent IO pulls may be (compute-local writes and lifecycle must stay sequential)", name)
		}
		delete(wantParallel, name)
	}
	for name := range wantParallel {
		t.Errorf("independent IO step %q is not Parallel; IO/compute split lost", name)
	}
}

func runPipelineSteps(t *testing.T, ctx context.Context, p *pipeline.Pipeline) ([]pipeline.Event, error) {
	t.Helper()
	var evs []pipeline.Event
	collected := make(chan struct{})
	go func() {
		for ev := range p.Events() {
			evs = append(evs, ev)
		}
		close(collected)
	}()
	err := p.Run(ctx)
	<-collected
	return evs, err
}
