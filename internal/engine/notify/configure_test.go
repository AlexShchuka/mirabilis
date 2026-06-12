package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

func updatesServer(t *testing.T, chatID int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{
					"channel_post": map[string]any{
						"chat": map[string]any{"id": chatID},
					},
				},
			},
		})
	}))
}

func TestConfigureEndToEnd(t *testing.T) {
	t.Parallel()
	const token = "tok-configure"
	const chatID = int64(-100999)
	repo := t.TempDir()
	store := secrets.NewFileStore(t.TempDir())
	srv := updatesServer(t, chatID)
	defer srv.Close()

	if err := Configure(context.Background(), store, srv.URL, repo, token); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	storedToken, err := store.Get(context.Background(), TokenKey)
	if err != nil {
		t.Fatalf("token not stored: %v", err)
	}
	if storedToken != token {
		t.Errorf("stored token = %q, want %q", storedToken, token)
	}

	written, err := ReadChatID(repo)
	if err != nil {
		t.Fatalf("chat-id not written: %v", err)
	}
	if written != "-100999" {
		t.Errorf("chat-id = %q, want -100999", written)
	}

	_, err = os.Stat(ChatIDPath(repo))
	if err != nil {
		t.Errorf("chat-id file does not exist: %v", err)
	}
}

type blockingStore struct {
	release chan struct{}
	done    chan struct{}
}

func (s *blockingStore) Get(context.Context, string) (string, error) {
	return "", errors.New("blocking store: no value")
}

func (s *blockingStore) Set(ctx context.Context, _, _ string) error {
	defer close(s.done)
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestConfigureCtxCancel(t *testing.T) {
	t.Parallel()
	store := &blockingStore{release: make(chan struct{}), done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Configure(ctx, store, "http://127.0.0.1:1", t.TempDir(), "tok-cancel")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Configure with canceled ctx = %v, want context.Canceled", err)
	}

	close(store.release)
	select {
	case <-store.done:
	case <-time.After(5 * time.Second):
		t.Fatal("detached token write did not finish after release")
	}
}

func TestConfigureDetectError(t *testing.T) {
	t.Parallel()
	store := secrets.NewFileStore(t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": []any{},
		})
	}))
	defer srv.Close()

	err := Configure(context.Background(), store, srv.URL, t.TempDir(), "tok-no-channel")
	if !errors.Is(err, ErrNoChannel) {
		t.Errorf("Configure with empty updates = %v, want ErrNoChannel", err)
	}
}
