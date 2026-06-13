package authproxy

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const testToken = "sk-ant-oat01-REALSECRETVALUE0123456789"

type staticToken struct {
	err   error
	token string
}

func (s staticToken) Token(context.Context) (string, error) { return s.token, s.err }

type switchableToken struct {
	mu          sync.Mutex
	token       string
	err         error
	invalidated int
}

func (s *switchableToken) Token(_ context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token, s.err
}

func (s *switchableToken) set(token string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
	s.err = err
}

func (s *switchableToken) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidated++
	s.token = ""
}

func (s *switchableToken) invalidations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.invalidated
}

func newTestObs(t *testing.T) (*obs.Obs, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host.log")
	o, err := obs.New(path)
	if err != nil {
		t.Fatalf("obs.New: %v", err)
	}
	t.Cleanup(func() { _ = o.Close() })
	return o, path
}

func startProxy(t *testing.T, o *obs.Obs, upstream string) (*Proxy, context.CancelFunc) {
	t.Helper()
	p := New(staticToken{token: testToken}, o, 0, "")
	u, err := url.Parse(upstream)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	p.upstream = u
	ctx, cancel := context.WithCancel(context.Background())
	if err := p.Start(ctx); err != nil {
		cancel()
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		waitDone(t, p)
	})
	return p, cancel
}

func waitDone(t *testing.T, p *Proxy) {
	t.Helper()
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not shut down")
	}
}

func doRequest(t *testing.T, p *Proxy, auth, beta string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://"+p.Addr()+"/v1/messages", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if beta != "" {
		req.Header.Set("anthropic-beta", beta)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestKey(t *testing.T) {
	o, _ := newTestObs(t)
	p := New(staticToken{token: testToken}, o, 0, "")
	key := p.Key()
	if len(key) < 32 {
		t.Fatalf("key length = %d, want >= 32", len(key))
	}
	if _, err := hex.DecodeString(key); err != nil {
		t.Fatalf("key is not hex: %v", err)
	}
	if p.Key() != key {
		t.Fatal("Key() not stable across calls")
	}
	if New(staticToken{token: testToken}, o, 0, "").Key() == key {
		t.Fatal("two proxies share a session key")
	}
}

func TestKeyProvided(t *testing.T) {
	o, _ := newTestObs(t)
	const provided = "deadbeefdeadbeefdeadbeefdeadbeef"
	p := New(staticToken{token: testToken}, o, 0, provided)
	if p.Key() != provided {
		t.Fatalf("Key() = %q, want %q", p.Key(), provided)
	}
}

func TestKeyProvidedServed(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	const provided = "feedfacefeedfacefeedfacefeedface"
	p := New(staticToken{token: testToken}, o, 0, provided)
	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	p.upstream = u
	ctx, cancel := context.WithCancel(context.Background())
	if err := p.Start(ctx); err != nil {
		cancel()
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		waitDone(t, p)
	})
	if got := doRequest(t, p, "Bearer "+provided, "").StatusCode; got != http.StatusOK {
		t.Fatalf("provided-key request status = %d, want 200", got)
	}
	if got := doRequest(t, p, "Bearer wrong-key", "").StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("wrong-key request status = %d, want 401", got)
	}
}

func TestBindHost(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: "127.0.0.1"},
		{goos: "linux", want: "0.0.0.0"},
		{goos: "windows", want: "127.0.0.1"},
	}
	for _, tt := range tests {
		if got := bindHost(tt.goos); got != tt.want {
			t.Errorf("bindHost(%q) = %q, want %q", tt.goos, got, tt.want)
		}
	}
}

func TestStartSucceedsWithoutToken(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	ts := &switchableToken{}
	p := New(ts, o, 0, "")
	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	p.upstream = u
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); waitDone(t, p) })

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start with no token = %v, want nil", err)
	}
	st := o.Snapshot()[node]
	if st.State != obs.StateOK || !strings.HasPrefix(st.Detail, "listening :") {
		t.Fatalf("proxy state after Start = %v %q, want ok listening", st.State, st.Detail)
	}
}

func TestTokenAbsentAtBootRequestGets503(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream reached despite missing token")
	}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	ts := &switchableToken{}
	p := New(ts, o, 0, "")
	u, _ := url.Parse(upstream.URL)
	p.upstream = u
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); waitDone(t, p) })
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	resp := doRequest(t, p, "Bearer "+p.Key(), "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	st := o.Snapshot()[node]
	if st.State != obs.StateDegraded {
		t.Fatalf("proxy state = %v, want degraded", st.State)
	}
}

func TestTokenAppearsLaterSelfHeals(t *testing.T) {
	var gotAuth atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	ts := &switchableToken{}
	p := New(ts, o, 0, "")
	u, _ := url.Parse(upstream.URL)
	p.upstream = u
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); waitDone(t, p) })
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	resp1 := doRequest(t, p, "Bearer "+p.Key(), "")
	if resp1.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("first request status = %d, want 503 (no token yet)", resp1.StatusCode)
	}

	ts.set(testToken, nil)

	resp2 := doRequest(t, p, "Bearer "+p.Key(), "")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second request status = %d, want 200 (token now available)", resp2.StatusCode)
	}
	if got := gotAuth.Load(); got != "Bearer "+testToken {
		t.Fatalf("upstream Authorization = %v, want Bearer <token>", got)
	}
}

func TestUpstream401InvalidatesToken(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "oauth rejected", http.StatusUnauthorized)
	}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	ts := &switchableToken{token: testToken}
	p := New(ts, o, 0, "")
	u, _ := url.Parse(upstream.URL)
	p.upstream = u
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); waitDone(t, p) })
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	resp := doRequest(t, p, "Bearer "+p.Key(), "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (upstream rejected)", resp.StatusCode)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && ts.invalidations() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := ts.invalidations(); got != 1 {
		t.Fatalf("invalidations = %d, want 1 (rotation: a rejected token must be evicted)", got)
	}
	if st := o.Snapshot()[node]; st.State != obs.StateDegraded {
		t.Fatalf("proxy state = %v, want degraded after upstream 401", st.State)
	}
}

func TestReadHeaderTimeoutSet(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	p, _ := startProxy(t, o, upstream.URL)
	_ = p
	if readHeaderTimeout == 0 {
		t.Fatal("readHeaderTimeout must be non-zero")
	}
	if readTimeout == 0 {
		t.Fatal("readTimeout must be non-zero")
	}
	if idleTimeout == 0 {
		t.Fatal("idleTimeout must be non-zero")
	}
}

func TestAuthorizationInjection(t *testing.T) {
	var gotAuth atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		fmt.Fprint(w, "upstream-ok")
	}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	p, _ := startProxy(t, o, upstream.URL)

	resp := doRequest(t, p, "Bearer "+p.Key(), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "upstream-ok" {
		t.Fatalf("body = (%q, %v), want upstream-ok", body, err)
	}
	if got := gotAuth.Load(); got != "Bearer "+testToken {
		t.Fatalf("upstream Authorization = %v, want Bearer <token>", got)
	}
	st := o.Snapshot()[node]
	if st.State != obs.StateOK {
		t.Fatalf("proxy state = %v, want ok", st.State)
	}
}

func TestRejectsBadSessionKey(t *testing.T) {
	var hits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	p, _ := startProxy(t, o, upstream.URL)

	for name, auth := range map[string]string{
		"missing": "",
		"wrong":   "Bearer deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	} {
		resp := doRequest(t, p, auth, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s key: status = %d, want 401", name, resp.StatusCode)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits.Load())
	}
}

func TestBetaHeaderUntouched(t *testing.T) {
	type seen struct {
		values  []string
		present bool
	}
	var got atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, present := r.Header["Anthropic-Beta"]
		got.Store(seen{values: v, present: present})
	}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	p, _ := startProxy(t, o, upstream.URL)

	doRequest(t, p, "Bearer "+p.Key(), "context-1m-2025-08-07,effort-2025-11-24")
	s := got.Load().(seen)
	if !s.present || len(s.values) != 1 || s.values[0] != "context-1m-2025-08-07,effort-2025-11-24" {
		t.Fatalf("upstream beta = %+v, want exactly the client value", s)
	}

	doRequest(t, p, "Bearer "+p.Key(), "")
	s = got.Load().(seen)
	if s.present {
		t.Fatalf("upstream beta = %+v, want absent", s)
	}
}

func TestSSEPassThrough(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Error("upstream writer is not a flusher")
			return
		}
		for i := range 3 {
			fmt.Fprintf(w, "data: chunk%d\n\n", i)
			fl.Flush()
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
	}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	p, _ := startProxy(t, o, upstream.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+p.Addr()+"/v1/messages", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.Key())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	sc := bufio.NewScanner(resp.Body)
	for i := range 3 {
		if !sc.Scan() {
			t.Fatalf("chunk %d not delivered before upstream completed: %v", i, sc.Err())
		}
		if got, want := sc.Text(), fmt.Sprintf("data: chunk%d", i); got != want {
			t.Fatalf("chunk %d = %q, want %q", i, got, want)
		}
		if !sc.Scan() || sc.Text() != "" {
			t.Fatalf("chunk %d: missing blank separator: %v", i, sc.Err())
		}
		release <- struct{}{}
	}
}

func TestSecretsAbsentFromLogs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(upstream.Close)
	o, logPath := newTestObs(t)
	p, cancel := startProxy(t, o, upstream.URL)

	doRequest(t, p, "Bearer "+p.Key(), "context-1m-2025-08-07")
	doRequest(t, p, "Bearer wrong-key", "")
	doRequest(t, p, "", "")
	cancel()
	waitDone(t, p)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	logs := string(data)
	if !strings.Contains(logs, "listening") || !strings.Contains(logs, "request") {
		t.Fatalf("log file missing expected entries:\n%s", logs)
	}
	if strings.Contains(logs, testToken) {
		t.Fatal("token leaked into log output")
	}
	if strings.Contains(logs, p.Key()) {
		t.Fatal("session key leaked into log output")
	}
}

func TestShutdownOnCancel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(upstream.Close)
	o, _ := newTestObs(t)
	p, cancel := startProxy(t, o, upstream.URL)
	addr := p.Addr()

	doRequest(t, p, "Bearer "+p.Key(), "")
	cancel()
	waitDone(t, p)

	st := o.Snapshot()[node]
	if st.State != obs.StateOff {
		t.Fatalf("proxy state = %v, want off", st.State)
	}
	if _, err := http.Get("http://" + addr + "/"); err == nil {
		t.Fatal("proxy still accepting connections after shutdown")
	}
}
