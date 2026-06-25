package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/obs"
)

func newTestChecker(srv *httptest.Server) *Checker {
	return &Checker{client: srv.Client(), baseURL: srv.URL}
}

func releaseHandler(tag, target string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"tag_name":         tag,
			"target_commitish": target,
		})
	}
}

func TestCheckCurrentIsLatest(t *testing.T) {
	srv := httptest.NewServer(releaseHandler("v1.2.3", "e641027abc123def"))
	defer srv.Close()

	tag, behind, err := newTestChecker(srv).Check(context.Background(), "e641027")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if behind {
		t.Errorf("behind = true, want false when current SHA is the release commit")
	}
	if tag != "v1.2.3" {
		t.Errorf("tag = %q, want v1.2.3", tag)
	}
}

func TestCheckBehindLatest(t *testing.T) {
	srv := httptest.NewServer(releaseHandler("v2.0.0", "abcdef0123456789"))
	defer srv.Close()

	tag, behind, err := newTestChecker(srv).Check(context.Background(), "e641027")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !behind {
		t.Errorf("behind = false, want true when current SHA differs from release commit")
	}
	if tag != "v2.0.0" {
		t.Errorf("tag = %q, want v2.0.0", tag)
	}
}

func TestCheckNetworkError(t *testing.T) {
	srv := httptest.NewServer(releaseHandler("v1.0.0", "deadbeef"))
	srv.Close()

	_, behind, err := newTestChecker(srv).Check(context.Background(), "e641027")
	if err == nil {
		t.Fatal("Check() error = nil, want network error")
	}
	if behind {
		t.Errorf("behind = true on network error, want false (graceful degradation)")
	}
}

func TestCheckUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, _, err := newTestChecker(srv).Check(context.Background(), "e641027")
	if err == nil {
		t.Fatal("Check() error = nil, want error on 404")
	}
}

func TestCheckMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, _, err := newTestChecker(srv).Check(context.Background(), "e641027")
	if err == nil {
		t.Fatal("Check() error = nil, want decode error on malformed JSON")
	}
}

func TestCheckEmptyTag(t *testing.T) {
	srv := httptest.NewServer(releaseHandler("", "deadbeef"))
	defer srv.Close()

	_, _, err := newTestChecker(srv).Check(context.Background(), "e641027")
	if err == nil {
		t.Fatal("Check() error = nil, want error on empty tag")
	}
}

func TestCheckContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(time.Second)
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := newTestChecker(srv).Check(ctx, "e641027")
	if err == nil {
		t.Fatal("Check() error = nil, want context cancellation error")
	}
}

func TestIsBehind(t *testing.T) {
	tests := []struct {
		name    string
		current string
		release string
		want    bool
	}{
		{"prefix match", "e641027", "e641027abc", false},
		{"exact match", "abc123", "abc123", false},
		{"different", "e641027", "9e58a6c", true},
		{"empty current", "", "deadbeef", false},
		{"empty release", "deadbeef", "", false},
		{"whitespace prefix match", " e641027 ", "e641027abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBehind(tt.current, tt.release); got != tt.want {
				t.Errorf("isBehind(%q, %q) = %v, want %v", tt.current, tt.release, got, tt.want)
			}
		})
	}
}

func newTestObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, err := obs.New(filepath.Join(t.TempDir(), "obs.log"))
	if err != nil {
		t.Fatalf("obs.New() error = %v", err)
	}
	t.Cleanup(func() { _ = o.Close() })
	return o
}

func TestRunSetsDegradedWhenBehind(t *testing.T) {
	srv := httptest.NewServer(releaseHandler("v9.9.9", "abcdef0123456789"))
	defer srv.Close()

	o := newTestObs(t)
	newTestChecker(srv).Run(context.Background(), o, "e641027")

	st, ok := o.Snapshot()[Node]
	if !ok {
		t.Fatalf("obs node %q not set when behind", Node)
	}
	if st.State != obs.StateDegraded {
		t.Errorf("state = %v, want StateDegraded", st.State)
	}
	if st.Detail != "v9.9.9" {
		t.Errorf("detail = %q, want v9.9.9", st.Detail)
	}
}

func TestRunSilentWhenCurrent(t *testing.T) {
	srv := httptest.NewServer(releaseHandler("v1.0.0", "e641027abc"))
	defer srv.Close()

	o := newTestObs(t)
	newTestChecker(srv).Run(context.Background(), o, "e641027")

	if _, ok := o.Snapshot()[Node]; ok {
		t.Errorf("obs node %q set when up to date, want silent (no badge)", Node)
	}
}

func TestRunSkipsUnknownVersion(t *testing.T) {
	var hits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer srv.Close()

	o := newTestObs(t)
	for _, v := range []string{"", "unknown"} {
		newTestChecker(srv).Run(context.Background(), o, v)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits != 0 {
		t.Errorf("server hit %d times for unknown/empty version, want 0 (no check)", hits)
	}
	if _, ok := o.Snapshot()[Node]; ok {
		t.Errorf("obs node set for unknown version, want silent")
	}
}

func TestRunReturnsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	o := newTestObs(t)
	done := make(chan struct{})
	go func() {
		newTestChecker(srv).Run(ctx, o, "e641027")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return promptly on cancelled context (goroutine leak risk)")
	}
}
