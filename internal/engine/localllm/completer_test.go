package localllm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newAdapter(t *testing.T, srv *httptest.Server) *HTTPAdapter {
	t.Helper()
	return &HTTPAdapter{
		BaseURL:   srv.URL,
		Model:     "test-model",
		Timeout:   5 * time.Second,
		MaxTokens: 256,
		Client:    srv.Client(),
	}
}

func fakeOK(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := chatResponse{
			Choices: []chatChoice{{Message: chatMessage{Role: "assistant", Content: text}}},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func TestHTTPAdapterSuccess(t *testing.T) {
	srv := httptest.NewServer(fakeOK("hello world"))
	defer srv.Close()

	a := newAdapter(t, srv)
	got, err := a.Complete(context.Background(), "say hi", Opts{})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "hello world" {
		t.Errorf("text = %q, want %q", got, "hello world")
	}
}

func TestHTTPAdapterSystemMessage(t *testing.T) {
	var captured chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := chatResponse{Choices: []chatChoice{{Message: chatMessage{Role: "assistant", Content: "ok"}}}}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	a := newAdapter(t, srv)
	if _, err := a.Complete(context.Background(), "hello", Opts{System: "be brief"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(captured.Messages) != 2 || captured.Messages[0].Role != "system" {
		t.Errorf("messages = %+v, want system+user pair", captured.Messages)
	}
	if captured.Messages[0].Content != "be brief" {
		t.Errorf("system content = %q, want %q", captured.Messages[0].Content, "be brief")
	}
}

func TestHTTPAdapterEndpointDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close()

	a := &HTTPAdapter{
		BaseURL:   url,
		Model:     "x",
		Timeout:   2 * time.Second,
		MaxTokens: 64,
		Client:    &http.Client{},
	}
	_, err := a.Complete(context.Background(), "ping", Opts{})
	if err == nil {
		t.Fatal("expected error for down endpoint, got nil")
	}
	if !strings.Contains(err.Error(), "local model unavailable at") {
		t.Errorf("error %q does not contain degraded sentinel", err.Error())
	}
}

func TestHTTPAdapterTimeout(t *testing.T) {
	ready := make(chan struct{})
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(ready)
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	t.Cleanup(func() {
		close(done)
		srv.Close()
	})

	a := &HTTPAdapter{
		BaseURL:   srv.URL,
		Model:     "x",
		Timeout:   100 * time.Millisecond,
		MaxTokens: 64,
		Client:    srv.Client(),
	}
	_, err := a.Complete(context.Background(), "hang", Opts{})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "local model unavailable at") {
		t.Errorf("error %q does not contain degraded sentinel", err.Error())
	}
}

func TestHTTPAdapterModelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := chatResponse{Error: &struct {
			Message string `json:"message"`
		}{Message: "model not loaded"}}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	a := newAdapter(t, srv)
	_, err := a.Complete(context.Background(), "hello", Opts{})
	if err == nil || !strings.Contains(err.Error(), "model not loaded") {
		t.Errorf("expected model error, got %v", err)
	}
}

func TestHTTPAdapterOptMaxTokensOverride(t *testing.T) {
	var captured chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := chatResponse{Choices: []chatChoice{{Message: chatMessage{Role: "assistant", Content: "x"}}}}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	a := newAdapter(t, srv)
	if _, err := a.Complete(context.Background(), "hi", Opts{MaxTokens: 999}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if captured.MaxTokens != 999 {
		t.Errorf("max_tokens = %d, want 999", captured.MaxTokens)
	}
}
