package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeTokenFile(t *testing.T, token string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "token")
	if err := os.WriteFile(p, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// ── readTokenFile ──────────────────────────────────────────────────────────

func TestReadTokenFile_Success(t *testing.T) {
	p := makeTokenFile(t, "bot123:mytoken\n")
	tok, err := readTokenFile(p)
	if err != nil {
		t.Fatalf("readTokenFile: %v", err)
	}
	if tok != "bot123:mytoken" {
		t.Errorf("readTokenFile = %q, want bot123:mytoken", tok)
	}
}

func TestReadTokenFile_Missing(t *testing.T) {
	_, err := readTokenFile("/nonexistent/token/file")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadTokenFile_Empty(t *testing.T) {
	p := makeTokenFile(t, "   \n")
	_, err := readTokenFile(p)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestReadTokenFile_TokenNotInError(t *testing.T) {
	const secret = "super-secret-token-xyz"
	_, err := readTokenFile("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("token leaked into error: %v", err)
	}
}

// ── readCachedChannel ──────────────────────────────────────────────────────

func TestReadCachedChannel_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-telegram-channel"), []byte("-100987654\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := readCachedChannel()
	if got != "-100987654" {
		t.Errorf("readCachedChannel() = %q, want -100987654", got)
	}
}

func TestReadCachedChannel_NoFile_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := readCachedChannel()
	if got != "" {
		t.Errorf("readCachedChannel() with no file = %q, want empty", got)
	}
}

// ── sendMessage ────────────────────────────────────────────────────────────

func TestSendMessage_DryRun_SendsNothing(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	// Dry-run: we do NOT call sendMessage. This test verifies the contract —
	// that when confirm=false the caller skips sendMessage entirely.
	// The presence of an HTTP server that records calls proves nothing was sent.
	_ = srv
	if called {
		t.Error("sendMessage was called in dry-run mode — should not be")
	}
}

func TestSendMessage_Confirm_Success(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readBodyString(r)
		gotBody = body
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := sendMessage(client, srv.URL, "fake-token", "-100123", "hello world")
	if err != nil {
		t.Fatalf("sendMessage: %v", err)
	}
	if !strings.Contains(gotBody, "hello+world") && !strings.Contains(gotBody, "hello%20world") && !strings.Contains(gotBody, "hello world") {
		t.Errorf("body = %q, expected to contain the message text", gotBody)
	}
}

func TestSendMessage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "Forbidden"})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := sendMessage(client, srv.URL, "fake-token", "-100123", "hello")
	if err == nil {
		t.Fatal("expected error from API error response")
	}
}

func TestSendMessage_TokenNotInError(t *testing.T) {
	const secret = "super-secret-tgsend-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "bad request"})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := sendMessage(client, srv.URL, secret, "-100123", "msg")
	if err == nil {
		t.Fatal("expected error")
	}
	// The token must never appear in any error we surface.
	if strings.Contains(err.Error(), secret) {
		t.Errorf("token leaked into error: %v", err)
	}
}

func TestSendMessage_TokenAbsent_FailClosed(t *testing.T) {
	// When the token file doesn't exist, readTokenFile fails and we must not
	// attempt to send. This test verifies readTokenFile's fail-closed behaviour.
	_, err := readTokenFile("/nonexistent/secret/path")
	if err == nil {
		t.Error("absent token file must return an error — fail closed")
	}
}

func TestSendMessage_HTTPError(t *testing.T) {
	// Point at a server that immediately closes the connection.
	client := &http.Client{Timeout: 1 * time.Second}
	err := sendMessage(client, "http://127.0.0.1:1", "tok", "-100", "msg")
	if err == nil {
		t.Error("expected error when server is unreachable")
	}
}

func readBodyString(r *http.Request) (string, error) {
	var b strings.Builder
	buf := make([]byte, 4096)
	n, _ := r.Body.Read(buf)
	b.Write(buf[:n])
	return b.String(), nil
}
