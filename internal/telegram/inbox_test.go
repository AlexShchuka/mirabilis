package telegram

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeUpdatesServer(t *testing.T, updates []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": updates,
		})
	}))
}

func TestInbox_NewInboxMirrorRequired(t *testing.T) {
	_, err := NewInbox("", "")
	if err == nil {
		t.Fatal("expected error when mirrorPath is empty")
	}
}

func TestInbox_PollChannelPosts(t *testing.T) {
	const msg1 = "hello world"
	const msg2 = "second message"
	sum1 := sha256.Sum256([]byte(msg1))
	sum2 := sha256.Sum256([]byte(msg2))

	updates := []map[string]any{
		{
			"update_id": 100,
			"channel_post": map[string]any{
				"message_id": 1,
				"date":       int64(1700000000),
				"text":       msg1,
			},
		},
		{
			"update_id": 101,
			"channel_post": map[string]any{
				"message_id": 2,
				"date":       int64(1700000001),
				"text":       msg2,
			},
		},
	}

	srv := makeUpdatesServer(t, updates)
	defer srv.Close()

	tok := writeTokenFile(t, "tok-inbox")
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	inbox, err := NewInbox(tok, mirror)
	if err != nil {
		t.Fatal(err)
	}
	inbox.withBaseURL(srv.URL)

	msgs, newOffset, err := inbox.Poll(context.Background(), 0)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if newOffset != 102 {
		t.Errorf("newOffset = %d, want 102", newOffset)
	}
	if msgs[0].TextHash != fmt.Sprintf("%x", sum1) {
		t.Errorf("hash[0] = %q, want %x", msgs[0].TextHash, sum1)
	}
	if msgs[1].TextHash != fmt.Sprintf("%x", sum2) {
		t.Errorf("hash[1] = %q, want %x", msgs[1].TextHash, sum2)
	}
}

func TestInbox_SkipsNonChannelPost(t *testing.T) {
	updates := []map[string]any{
		{
			"update_id": 200,
			"message": map[string]any{
				"message_id": 9,
				"date":       int64(1700000000),
				"text":       "should be ignored",
			},
		},
	}

	srv := makeUpdatesServer(t, updates)
	defer srv.Close()

	tok := writeTokenFile(t, "tok-skip")
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	inbox, err := NewInbox(tok, mirror)
	if err != nil {
		t.Fatal(err)
	}
	inbox.withBaseURL(srv.URL)

	msgs, _, err := inbox.Poll(context.Background(), 0)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages (non-channel_post filtered), got %d", len(msgs))
	}
}

func TestInbox_AppendWritesJSONL(t *testing.T) {
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	inbox, err := NewInbox(writeTokenFile(t, "tok"), mirror)
	if err != nil {
		t.Fatal(err)
	}

	msgs := []Message{
		{MessageID: 1, Date: 1700000000, TextHash: "aabbcc", Text: "first"},
		{MessageID: 2, Date: 1700000001, TextHash: "ddeeff", Text: "second"},
	}
	if err := inbox.Append(msgs); err != nil {
		t.Fatalf("Append: %v", err)
	}

	f, err := os.Open(mirror)
	if err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []Message
	for scanner.Scan() {
		var m Message
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		lines = append(lines, m)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Text != "first" || lines[1].Text != "second" {
		t.Errorf("unexpected line content: %+v", lines)
	}
}

func TestInbox_AppendIsAppendOnly(t *testing.T) {
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	inbox, err := NewInbox(writeTokenFile(t, "tok"), mirror)
	if err != nil {
		t.Fatal(err)
	}

	batch1 := []Message{{MessageID: 1, Date: 1700000000, TextHash: "aa", Text: "one"}}
	batch2 := []Message{{MessageID: 2, Date: 1700000001, TextHash: "bb", Text: "two"}}

	if err := inbox.Append(batch1); err != nil {
		t.Fatal(err)
	}
	if err := inbox.Append(batch2); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(mirror)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}
	if lineCount != 2 {
		t.Errorf("append-only: expected 2 lines total, got %d", lineCount)
	}
}

func TestInbox_TokenNeverInError(t *testing.T) {
	const secret = "inbox-secret-token-value"
	tok := writeTokenFile(t, secret)
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false})
	}))
	defer srv.Close()

	inbox, err := NewInbox(tok, mirror)
	if err != nil {
		t.Fatal(err)
	}
	inbox.withBaseURL(srv.URL)

	_, _, err = inbox.Poll(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error from mocked server")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("token leaked into error message: %v", err)
	}
}

func TestNewInbox_EmptyTokenPathDefaultsToConstant(t *testing.T) {
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")
	inbox, err := NewInbox("", mirror)
	if err != nil {
		t.Fatal(err)
	}
	if inbox.tokenPath != DefaultTokenPath {
		t.Errorf("tokenPath = %q, want %q", inbox.tokenPath, DefaultTokenPath)
	}
}

func TestInbox_Poll_TokenReadError(t *testing.T) {
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")
	inbox, err := NewInbox("/nonexistent/token/path", mirror)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = inbox.Poll(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for missing token file")
	}
}

func TestInbox_Poll_WithPositiveOffset(t *testing.T) {
	updates := []map[string]any{
		{
			"update_id": 200,
			"channel_post": map[string]any{
				"message_id": 3,
				"date":       int64(1700000002),
				"text":       "offset test",
			},
		},
	}
	srv := makeUpdatesServer(t, updates)
	defer srv.Close()

	tok := writeTokenFile(t, "tok-offset")
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	inbox, err := NewInbox(tok, mirror)
	if err != nil {
		t.Fatal(err)
	}
	inbox.withBaseURL(srv.URL)

	msgs, newOffset, err := inbox.Poll(context.Background(), 5)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if newOffset != 201 {
		t.Errorf("newOffset = %d, want 201", newOffset)
	}
}

func TestInbox_Poll_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()

	tok := writeTokenFile(t, "tok-badjson")
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	inbox, err := NewInbox(tok, mirror)
	if err != nil {
		t.Fatal(err)
	}
	inbox.withBaseURL(srv.URL)

	_, _, err = inbox.Poll(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestInbox_Poll_HTTPError(t *testing.T) {
	tok := writeTokenFile(t, "tok-hterr")
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	inbox, err := NewInbox(tok, mirror)
	if err != nil {
		t.Fatal(err)
	}
	inbox.withBaseURL(srv.URL)

	_, _, err = inbox.Poll(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error when server is closed")
	}
}

func TestInbox_Append_EmptyNoOp(t *testing.T) {
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")
	inbox, err := NewInbox(writeTokenFile(t, "tok"), mirror)
	if err != nil {
		t.Fatal(err)
	}
	if err := inbox.Append(nil); err != nil {
		t.Fatalf("Append(nil) returned unexpected error: %v", err)
	}
}

func TestInbox_Append_OpenError(t *testing.T) {
	roDir := t.TempDir()
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o755) })

	mirrorPath := filepath.Join(roDir, "mirror.jsonl")
	inbox, err := NewInbox(writeTokenFile(t, "tok"), mirrorPath)
	if err != nil {
		t.Fatal(err)
	}
	msgs := []Message{{MessageID: 1, Date: 1700000000, TextHash: "aa", Text: "x"}}
	if err := inbox.Append(msgs); err == nil {
		t.Fatal("expected error when mirror dir is read-only")
	}
}

func TestInbox_ReadToken_EmptyFile(t *testing.T) {
	tok := writeTokenFile(t, "   \n")
	mirror := filepath.Join(t.TempDir(), "mirror.jsonl")
	inbox, err := NewInbox(tok, mirror)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = inbox.Poll(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for empty token file")
	}
}
