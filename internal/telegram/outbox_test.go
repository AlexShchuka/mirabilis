package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTokenFile(t *testing.T, token string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "token")
	if err := os.WriteFile(p, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func alwaysConfirm(_, _ string) bool { return true }

func neverConfirm(_, _ string) bool { return false }

func TestNewOutbox_NilConfirmRejected(t *testing.T) {
	_, err := NewOutbox("", nil)
	if err == nil {
		t.Fatal("expected error when confirm is nil")
	}
}

func TestOutbox_SendRefusedWithoutConfirm(t *testing.T) {
	tok := writeTokenFile(t, "test-token-value")
	ob, err := NewOutbox(tok, neverConfirm)
	if err != nil {
		t.Fatal(err)
	}
	err = ob.Send(context.Background(), "-100123", "hello")
	if err == nil {
		t.Fatal("Send must return error when confirm returns false")
	}
}

func TestOutbox_TokenNeverInError(t *testing.T) {
	const secret = "super-secret-bot-token"
	tok := writeTokenFile(t, secret)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "bad request"})
	}))
	defer srv.Close()

	ob, err := NewOutbox(tok, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	ob.withBaseURL(srv.URL)

	err = ob.Send(context.Background(), "-100123", "hello")
	if err == nil {
		t.Fatal("expected error from mocked server")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("token leaked into error: %v", err)
	}
}

func TestOutbox_SendSuccess(t *testing.T) {
	tok := writeTokenFile(t, "tok-abc")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	ob, err := NewOutbox(tok, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	ob.withBaseURL(srv.URL)

	if err := ob.Send(context.Background(), "-100123", "hello"); err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestOutbox_RateLimit(t *testing.T) {
	tok := writeTokenFile(t, "tok-rate")

	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	ob, err := NewOutbox(tok, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	ob.withBaseURL(srv.URL)

	start := time.Now()
	for i := 0; i < 2; i++ {
		if err := ob.Send(context.Background(), "-100123", "msg"); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	if count != 2 {
		t.Errorf("expected 2 requests, got %d", count)
	}
	if elapsed < maxRate {
		t.Errorf("two sends completed in %v, want >= %v (rate limit)", elapsed, maxRate)
	}
}

func TestOutbox_MissingTokenFile(t *testing.T) {
	ob, err := NewOutbox("/nonexistent/path/token", alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	err = ob.Send(context.Background(), "-100123", "hello")
	if err == nil {
		t.Fatal("expected error for missing token file")
	}
}

func TestNewOutbox_EmptyTokenPathDefaultsToConstant(t *testing.T) {
	ob, err := NewOutbox("", alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	if ob.tokenPath != DefaultTokenPath {
		t.Errorf("tokenPath = %q, want %q", ob.tokenPath, DefaultTokenPath)
	}
}

func TestOutbox_ReadToken_EmptyFile(t *testing.T) {
	tok := writeTokenFile(t, "   \n")
	ob, err := NewOutbox(tok, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	err = ob.Send(context.Background(), "-100123", "hello")
	if err == nil {
		t.Fatal("expected error for empty token file")
	}
}

func TestOutbox_DoSend_HTTPError(t *testing.T) {
	tok := writeTokenFile(t, "tok-hterr")
	ob, err := NewOutbox(tok, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	ob.withBaseURL(srv.URL)

	err = ob.Send(context.Background(), "-100123", "hello")
	if err == nil {
		t.Fatal("expected error when server is closed")
	}
}

func TestOutbox_DoSend_BadJSON(t *testing.T) {
	tok := writeTokenFile(t, "tok-badjson")
	ob, err := NewOutbox(tok, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()
	ob.withBaseURL(srv.URL)

	err = ob.Send(context.Background(), "-100123", "hello")
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestOutbox_Send_CtxCanceledDuringRateLimit(t *testing.T) {
	tok := writeTokenFile(t, "tok-ctx")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	ob, err := NewOutbox(tok, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	ob.withBaseURL(srv.URL)

	if err := ob.Send(context.Background(), "-100123", "first"); err != nil {
		t.Fatalf("first Send: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = ob.Send(ctx, "-100123", "second")
	if err == nil {
		t.Fatal("expected error when context is already canceled during rate-limit wait")
	}
}
