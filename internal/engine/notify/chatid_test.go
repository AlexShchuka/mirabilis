package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

func channelChat(id int64) map[string]any {
	return map[string]any{"id": id, "type": "channel"}
}

func TestChatIDPathShape(t *testing.T) {
	t.Parallel()
	got := ChatIDPath("/some/repo")
	want := filepath.Join("/some/repo", ".mirabilis", "chat-id")
	if got != want {
		t.Errorf("ChatIDPath = %q, want %q", got, want)
	}
}

func TestReadWriteChatIDRoundTrip(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	const id = "-100123456789"
	if err := WriteChatID(repo, id); err != nil {
		t.Fatalf("WriteChatID: %v", err)
	}
	got, err := ReadChatID(repo)
	if err != nil {
		t.Fatalf("ReadChatID: %v", err)
	}
	if got != id {
		t.Errorf("ReadChatID = %q, want %q", got, id)
	}
}

func TestReadChatIDTrimsWhitespace(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	path := ChatIDPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("  -100987  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadChatID(repo)
	if err != nil {
		t.Fatalf("ReadChatID: %v", err)
	}
	if got != "-100987" {
		t.Errorf("ReadChatID = %q, want -100987", got)
	}
}

func TestWriteChatIDTrimsAndAppendsNewline(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := WriteChatID(repo, "  -100111  "); err != nil {
		t.Fatalf("WriteChatID: %v", err)
	}
	raw, err := os.ReadFile(ChatIDPath(repo))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "-100111\n" {
		t.Errorf("file contents = %q, want \"-100111\\n\"", string(raw))
	}
}

func TestReadChatIDMissingFileError(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	_, err := ReadChatID(repo)
	if err == nil {
		t.Fatal("ReadChatID on missing file = nil, want error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ReadChatID error = %v, want os.ErrNotExist", err)
	}
}

func TestDetectChatIDHappyPath(t *testing.T) {
	t.Parallel()
	const token = "tok-detect"
	store := storeWithToken(t, token)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{
					"channel_post": map[string]any{
						"chat": channelChat(-100567),
					},
				},
			},
		})
	}))
	defer srv.Close()

	got, err := DetectChatID(context.Background(), store, srv.URL)
	if err != nil {
		t.Fatalf("DetectChatID: %v", err)
	}
	if got != "-100567" {
		t.Errorf("DetectChatID = %q, want -100567", got)
	}
}

func TestDetectChatIDMyChatMember(t *testing.T) {
	t.Parallel()
	store := storeWithToken(t, "tok-mcm")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{
					"my_chat_member": map[string]any{
						"chat": channelChat(-100999),
					},
				},
			},
		})
	}))
	defer srv.Close()

	got, err := DetectChatID(context.Background(), store, srv.URL)
	if err != nil {
		t.Fatalf("DetectChatID (my_chat_member): %v", err)
	}
	if got != "-100999" {
		t.Errorf("DetectChatID = %q, want -100999", got)
	}
}

func TestDetectChatIDMultiUpdate(t *testing.T) {
	t.Parallel()
	store := storeWithToken(t, "tok-multi")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{"message": map[string]any{"chat": map[string]any{"id": int64(-100111), "type": "group"}}},
				{"channel_post": map[string]any{"chat": channelChat(-100222)}},
			},
		})
	}))
	defer srv.Close()

	got, err := DetectChatID(context.Background(), store, srv.URL)
	if err != nil {
		t.Fatalf("DetectChatID (multi-update): %v", err)
	}
	if got != "-100222" {
		t.Errorf("DetectChatID = %q, want -100222", got)
	}
}

func TestDetectChatIDWebhookConflict(t *testing.T) {
	t.Parallel()
	store := storeWithToken(t, "tok-webhook")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"description": "Conflict: can't use getUpdates method while webhook is active",
		})
	}))
	defer srv.Close()

	_, err := DetectChatID(context.Background(), store, srv.URL)
	if !errors.Is(err, ErrWebhookConflict) {
		t.Errorf("DetectChatID webhook conflict = %v, want ErrWebhookConflict", err)
	}
}

func TestDetectChatIDNoUpdates(t *testing.T) {
	t.Parallel()
	store := storeWithToken(t, "tok-no-updates")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": []any{},
		})
	}))
	defer srv.Close()

	_, err := DetectChatID(context.Background(), store, srv.URL)
	if !errors.Is(err, ErrNoChannel) {
		t.Errorf("DetectChatID with no updates = %v, want ErrNoChannel", err)
	}
}

func TestDetectChatIDUpdatesWithNilChannelPost(t *testing.T) {
	t.Parallel()
	store := storeWithToken(t, "tok-nil-post")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{"some_other_field": "value"},
			},
		})
	}))
	defer srv.Close()

	_, err := DetectChatID(context.Background(), store, srv.URL)
	if !errors.Is(err, ErrNoChannel) {
		t.Errorf("DetectChatID with nil channel_post = %v, want ErrNoChannel", err)
	}
}

func TestDetectChatIDBadToken(t *testing.T) {
	t.Parallel()
	store := storeWithToken(t, "tok-bad")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"description": "Unauthorized",
		})
	}))
	defer srv.Close()

	_, err := DetectChatID(context.Background(), store, srv.URL)
	if err == nil {
		t.Fatal("DetectChatID with bad token = nil, want error")
	}
}

func TestDetectChatIDTokenStoreError(t *testing.T) {
	t.Parallel()
	store := secrets.NewFileStore(t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	_, err := DetectChatID(context.Background(), store, srv.URL)
	if err == nil {
		t.Fatal("DetectChatID with no token in store = nil, want error")
	}
}

func TestDetectChatIDMalformedJSON(t *testing.T) {
	t.Parallel()
	store := storeWithToken(t, "tok-malformed")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()

	_, err := DetectChatID(context.Background(), store, srv.URL)
	if !errors.Is(err, ErrNoChannel) {
		t.Errorf("DetectChatID with malformed JSON = %v, want ErrNoChannel", err)
	}
}
