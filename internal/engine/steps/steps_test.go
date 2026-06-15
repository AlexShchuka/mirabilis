package steps

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/claudeauth"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const (
	testProxyAddr  = "http://host.docker.internal:8788"
	testSessionKey = "test-session-key-1234"
)

type fakeStore struct {
	mu sync.Mutex
	m  map[string]string
}

var _ secrets.Store = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{m: map[string]string{}}
}

func (f *fakeStore) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.m[key]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return v, nil
}

func (f *fakeStore) Set(_ context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = value
	return nil
}

type recordingRunner struct {
	inner exec.Runner

	mu    sync.Mutex
	specs []exec.Spec
}

func (r *recordingRunner) Stream(ctx context.Context, spec exec.Spec) <-chan exec.Event {
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	return r.inner.Stream(ctx, spec)
}

func (r *recordingRunner) Specs() []exec.Spec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]exec.Spec, len(r.specs))
	copy(out, r.specs)
	return out
}

func newTestDeps(t *testing.T, runner exec.Runner, docker sandbox.Docker, store secrets.Store) Deps {
	t.Helper()
	repo := t.TempDir()
	o, err := obs.New(filepath.Join(t.TempDir(), "host.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	return Deps{
		Runner:     runner,
		Docker:     docker,
		Sandbox:    sandbox.New(runner, docker, repo),
		Store:      store,
		Tokens:     claudeauth.NewSource(store),
		Obs:        o,
		ProxyAddr:  func() string { return testProxyAddr },
		SessionKey: func() string { return testSessionKey },
		Repo:       repo,
	}
}

func runStep(t *testing.T, step pipeline.Command, resolve func(payload any) pipeline.Result) ([]pipeline.Event, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(chan pipeline.Event)
	in := make(chan pipeline.Result)
	collected := make(chan []pipeline.Event, 1)
	go func() {
		var evs []pipeline.Event
		for ev := range out {
			evs = append(evs, ev)
			if ev.Kind == pipeline.EvWaiting && resolve != nil {
				r := resolve(ev.Payload)
				go func() {
					select {
					case in <- r:
					case <-ctx.Done():
					}
				}()
			}
		}
		collected <- evs
	}()
	err := step.Run(ctx, out, in)
	close(out)
	return <-collected, err
}

func waitingEvent(t *testing.T, evs []pipeline.Event) pipeline.Event {
	t.Helper()
	for _, ev := range evs {
		if ev.Kind == pipeline.EvWaiting {
			return ev
		}
	}
	t.Fatal("no EvWaiting event emitted")
	return pipeline.Event{}
}

func mustCheck(t *testing.T, step pipeline.Command, want bool) {
	t.Helper()
	got, err := step.Check(t.Context())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if got != want {
		t.Fatalf("check = %v, want %v", got, want)
	}
}
