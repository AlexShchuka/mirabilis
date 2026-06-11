package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testChatID = "-100123"

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

func newPinnedOutbox(t *testing.T, tokenPath string, confirm ConfirmFunc) *Outbox {
	t.Helper()
	ob, err := NewOutbox(tokenPath, testChatID, confirm)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}
	return ob
}

func TestNewOutbox_NilConfirmRejected(t *testing.T) {
	_, err := NewOutbox("", testChatID, nil)
	if err == nil {
		t.Fatal("expected error when confirm is nil")
	}
}

func TestOutbox_SendRefusedWithoutConfirm(t *testing.T) {
	tok := writeTokenFile(t, "test-token-value")
	ob := newPinnedOutbox(t, tok, neverConfirm)
	err := ob.Send(context.Background(), testChatID, "hello")
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

	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	ob.withBaseURL(srv.URL)

	err := ob.Send(context.Background(), testChatID, "hello")
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

	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	ob.withBaseURL(srv.URL)

	if err := ob.Send(context.Background(), testChatID, "hello"); err != nil {
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

	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	ob.withBaseURL(srv.URL)

	start := time.Now()
	for i := 0; i < 2; i++ {
		if err := ob.Send(context.Background(), testChatID, "msg"); err != nil {
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
	ob := newPinnedOutbox(t, "/nonexistent/path/token", alwaysConfirm)
	err := ob.Send(context.Background(), testChatID, "hello")
	if err == nil {
		t.Fatal("expected error for missing token file")
	}
}

func TestNewOutbox_EmptyTokenPathDefaultsToConstant(t *testing.T) {
	ob, err := NewOutbox("", testChatID, alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	if ob.tokenPath != DefaultTokenPath {
		t.Errorf("tokenPath = %q, want %q", ob.tokenPath, DefaultTokenPath)
	}
}

func TestOutbox_ReadToken_EmptyFile(t *testing.T) {
	tok := writeTokenFile(t, "   \n")
	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	err := ob.Send(context.Background(), testChatID, "hello")
	if err == nil {
		t.Fatal("expected error for empty token file")
	}
}

func TestOutbox_DoSend_HTTPError(t *testing.T) {
	tok := writeTokenFile(t, "tok-hterr")
	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	ob.withBaseURL(srv.URL)

	err := ob.Send(context.Background(), testChatID, "hello")
	if err == nil {
		t.Fatal("expected error when server is closed")
	}
}

func TestOutbox_DoSend_BadJSON(t *testing.T) {
	tok := writeTokenFile(t, "tok-badjson")
	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()
	ob.withBaseURL(srv.URL)

	err := ob.Send(context.Background(), testChatID, "hello")
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

	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	ob.withBaseURL(srv.URL)

	if err := ob.Send(context.Background(), testChatID, "first"); err != nil {
		t.Fatalf("first Send: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ob.Send(ctx, testChatID, "second")
	if err == nil {
		t.Fatal("expected error when context is already canceled during rate-limit wait")
	}
}

func TestOutbox_DoSend_InvalidBaseURL_BuildRequestFails(t *testing.T) {
	// A base URL containing a control character (e.g., \n) makes
	// http.NewRequestWithContext return an error before any network call.
	tok := writeTokenFile(t, "tok-invalidurl")
	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	ob.withBaseURL("http://invalid\x00host")

	err := ob.Send(context.Background(), testChatID, "hello")
	if err == nil {
		t.Fatal("Send with invalid base URL = nil, want error")
	}
}

// ── Channel pin tests ──────────────────────────────────────────────────────

func TestOutbox_ChannelPin_PinnedIDSucceeds(t *testing.T) {
	tok := writeTokenFile(t, "tok-pin")
	var serverCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	ob.withBaseURL(srv.URL)

	if err := ob.Send(context.Background(), testChatID, "hello"); err != nil {
		t.Fatalf("Send to pinned id = %v, want nil", err)
	}
	if serverCalls.Load() == 0 {
		t.Error("expected HTTP call for pinned chat_id send")
	}
}

func TestOutbox_ChannelPin_DifferentIDRejectedBeforeNetwork(t *testing.T) {
	tok := writeTokenFile(t, "tok-pin")
	var serverCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	ob := newPinnedOutbox(t, tok, alwaysConfirm)
	ob.withBaseURL(srv.URL)

	// Send to a DIFFERENT chat_id — must be rejected before any network call.
	err := ob.Send(context.Background(), "-999000999", "hello")
	if err == nil {
		t.Fatal("Send to non-pinned id = nil, want error")
	}
	if !strings.Contains(err.Error(), "refused chat_id") {
		t.Errorf("error = %q, want 'refused chat_id' message", err)
	}
	if serverCalls.Load() != 0 {
		t.Errorf("server was called %d times; must be 0 for rejected chat_id", serverCalls.Load())
	}
}

func TestOutbox_ChannelPin_EmptyAllowedIDRefusesAll(t *testing.T) {
	tok := writeTokenFile(t, "tok-pin")
	var serverCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	// Construct outbox with empty allowedChatID — all sends must be refused.
	ob, err := NewOutbox(tok, "", alwaysConfirm)
	if err != nil {
		t.Fatal(err)
	}
	ob.withBaseURL(srv.URL)

	err = ob.Send(context.Background(), testChatID, "hello")
	if err == nil {
		t.Fatal("Send with empty allowedChatID = nil, want error")
	}
	if !strings.Contains(err.Error(), "no pinned channel configured") {
		t.Errorf("error = %q, want 'no pinned channel configured' message", err)
	}
	if serverCalls.Load() != 0 {
		t.Errorf("server was called %d times; must be 0 when no channel is configured", serverCalls.Load())
	}
}
