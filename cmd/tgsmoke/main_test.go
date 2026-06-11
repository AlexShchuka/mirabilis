package main

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

func makeSmokeTokenFile(t *testing.T, token string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "token")
	if err := os.WriteFile(p, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadToken(t *testing.T) {
	p := makeSmokeTokenFile(t, "my-token\n")
	tok, err := readToken(p)
	if err != nil {
		t.Fatalf("readToken: %v", err)
	}
	if tok != "my-token" {
		t.Errorf("readToken = %q, want my-token", tok)
	}
}

func TestReadToken_Missing(t *testing.T) {
	_, err := readToken("/nonexistent/token")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadToken_Empty(t *testing.T) {
	p := makeSmokeTokenFile(t, "   \n")
	_, err := readToken(p)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestSendMessage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := sendMessage(client, srv.URL, "fake-token", "-100123", "hello")
	if err != nil {
		t.Fatalf("sendMessage: %v", err)
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
		t.Fatal("expected error")
	}
}

func TestPollForCanary_Found(t *testing.T) {
	const canary = "tgsmoke-canary-12345"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{
					"update_id": 1,
					"channel_post": map[string]any{
						"message_id": 10,
						"date":       1700000000,
						"text":       canary,
					},
				},
			},
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	found, err := pollForCanary(ctx, client, srv.URL, "fake-token", canary)
	if err != nil {
		t.Fatalf("pollForCanary: %v", err)
	}
	if !found {
		t.Error("expected canary to be found")
	}
}

func TestPollForCanary_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": []map[string]any{},
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	found, err := pollForCanary(ctx, client, srv.URL, "fake-token", "not-there")
	if err != nil {
		t.Fatalf("pollForCanary: %v", err)
	}
	if found {
		t.Error("expected canary NOT to be found (timeout)")
	}
}

func TestSendMessage_TokenNotInError(t *testing.T) {
	const secret = "super-secret-smoke-token"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "bad request"})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := sendMessage(client, srv.URL, secret, "-100123", "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("token leaked into error: %v", err)
	}
}

func TestSendMessage_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := sendMessage(client, srv.URL, "tok", "-100123", "hello")
	if err == nil {
		t.Fatal("sendMessage must return error on HTTP failure")
	}
}

func TestGetUpdates_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	_, _, err := getUpdates(client, srv.URL, "tok", 0)
	if err == nil {
		t.Fatal("getUpdates must return error when ok=false")
	}
}

func TestGetUpdates_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	_, _, err := getUpdates(client, srv.URL, "tok", 0)
	if err == nil {
		t.Fatal("getUpdates must return error on HTTP failure")
	}
}

func TestPollForCanary_GetUpdatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	_, err := pollForCanary(ctx, client, srv.URL, "tok", "canary")
	if err == nil {
		t.Fatal("pollForCanary must propagate getUpdates error")
	}
}

func TestGetUpdates_SkipsNonChannelPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{
					"update_id": 50,
					"message": map[string]any{
						"message_id": 5,
						"date":       1700000000,
						"text":       "ignored",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	texts, newOffset, err := getUpdates(client, srv.URL, "tok", 0)
	if err != nil {
		t.Fatalf("getUpdates: %v", err)
	}
	if len(texts) != 0 {
		t.Errorf("expected 0 texts from non-channel_post update, got %d", len(texts))
	}
	if newOffset != 51 {
		t.Errorf("newOffset = %d, want 51", newOffset)
	}
}
