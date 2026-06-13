package claudeauth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

const testToken = "sk-ant-oat01-0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghij_-AA"

type fakeStore struct {
	mu     sync.Mutex
	values map[string]string
	setErr error
	gets   int
	sets   int
}

var _ secrets.Store = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{values: make(map[string]string)}
}

func (f *fakeStore) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	v, ok := f.values[key]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return v, nil
}

func (f *fakeStore) Set(_ context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets++
	if f.setErr != nil {
		return f.setErr
	}
	f.values[key] = value
	return nil
}

func (f *fakeStore) getCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gets
}

func (f *fakeStore) put(key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[key] = value
}

func (f *fakeStore) value(key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.values[key]
	return v, ok
}

func TestSourceConcurrentReadsConverge(t *testing.T) {
	store := newFakeStore()
	store.put(tokenKey, testToken)
	src := NewSource(store)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := src.Token(context.Background())
			if err != nil {
				t.Errorf("Token() error = %v", err)
				return
			}
			if got != testToken {
				t.Errorf("Token() = %q, want %q", got, testToken)
			}
		}()
	}
	wg.Wait()

	if got := store.getCount(); got < 1 {
		t.Errorf("store gets = %d, want at least 1", got)
	}
	v, _ := src.Token(context.Background())
	if v != testToken {
		t.Errorf("cached Token() = %q, want %q", v, testToken)
	}
}

type gatedStore struct {
	mu       sync.Mutex
	inFlight int
	maxJoint int
	gets     int
	value    string
	release  chan struct{}
	gateOpen chan struct{}
	gateOnce sync.Once
}

var _ secrets.Store = (*gatedStore)(nil)

func newGatedStore(value string) *gatedStore {
	return &gatedStore{value: value, release: make(chan struct{}), gateOpen: make(chan struct{})}
}

func (g *gatedStore) Get(_ context.Context, _ string) (string, error) {
	g.mu.Lock()
	g.gets++
	g.inFlight++
	if g.inFlight > g.maxJoint {
		g.maxJoint = g.inFlight
	}
	if g.inFlight >= 2 {
		g.gateOnce.Do(func() { close(g.gateOpen) })
	}
	g.mu.Unlock()

	<-g.release

	g.mu.Lock()
	g.inFlight--
	g.mu.Unlock()
	return g.value, nil
}

func (g *gatedStore) Set(_ context.Context, _, _ string) error { return nil }

func TestSourceConcurrentReadsDoNotSerialize(t *testing.T) {
	store := newGatedStore(testToken)
	src := NewSource(store)

	const n = 4
	done := make(chan error, n)
	for range n {
		go func() {
			_, err := src.Token(context.Background())
			done <- err
		}()
	}

	select {
	case <-store.gateOpen:
	case <-time.After(2 * time.Second):
		t.Fatal("callers serialized behind store.Get; at most one reached Get at a time")
	}
	close(store.release)

	for range n {
		if err := <-done; err != nil {
			t.Errorf("Token() error = %v", err)
		}
	}
	if store.maxJoint < 2 {
		t.Errorf("max concurrent Get = %d, want >= 2 (callers must not serialize)", store.maxJoint)
	}
}

func TestSourceInvalidateRefetches(t *testing.T) {
	store := newFakeStore()
	store.put(tokenKey, testToken)
	src := NewSource(store)
	ctx := context.Background()

	if _, err := src.Token(ctx); err != nil {
		t.Fatalf("Token() = %v", err)
	}
	if gets := store.getCount(); gets != 1 {
		t.Fatalf("store gets = %d, want 1", gets)
	}
	src.(interface{ Invalidate() }).Invalidate()
	if _, err := src.Token(ctx); err != nil {
		t.Fatalf("Token() after invalidate = %v", err)
	}
	if gets := store.getCount(); gets != 2 {
		t.Fatalf("store gets after invalidate = %d, want 2", gets)
	}
}

func TestSourceNotFoundNotCached(t *testing.T) {
	store := newFakeStore()
	src := NewSource(store)
	ctx := context.Background()

	if _, err := src.Token(ctx); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("Token() error = %v, want ErrNotFound", err)
	}

	store.put(tokenKey, testToken)
	got, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() after login: error = %v", err)
	}
	if got != testToken {
		t.Fatalf("Token() = %q, want %q", got, testToken)
	}
	if gets := store.getCount(); gets != 2 {
		t.Fatalf("store gets = %d, want 2", gets)
	}

	if _, err := src.Token(ctx); err != nil {
		t.Fatalf("Token() cached: error = %v", err)
	}
	if gets := store.getCount(); gets != 2 {
		t.Fatalf("store gets after cached call = %d, want 2", gets)
	}
}

func TestSourceRejectsNonOATToken(t *testing.T) {
	const stored = "sk-ant-api03-supersecretapikeyvalue"
	store := newFakeStore()
	store.put(tokenKey, stored)
	src := NewSource(store)
	ctx := context.Background()

	_, err := src.Token(ctx)
	if err == nil {
		t.Fatal("Token() error = nil, want non-oat rejection")
	}
	if got, want := err.Error(), "stored claude token is not an oat token"; got != want {
		t.Fatalf("Token() error = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), stored) || strings.Contains(err.Error(), "supersecret") {
		t.Fatalf("Token() error %q leaks the stored value", err)
	}

	store.put(tokenKey, testToken)
	got, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() after fix: error = %v", err)
	}
	if got != testToken {
		t.Fatalf("Token() = %q, want %q", got, testToken)
	}
}
