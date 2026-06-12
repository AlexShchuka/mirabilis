package claudeauth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

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

func TestSourceConcurrentSingleRead(t *testing.T) {
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

	if got := store.getCount(); got != 1 {
		t.Errorf("store gets = %d, want 1", got)
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
