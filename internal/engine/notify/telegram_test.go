package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

type capturedRequest struct {
	path        string
	contentType string
	form        map[string]string
}

type captureServer struct {
	mu   sync.Mutex
	reqs []capturedRequest
}

func (c *captureServer) handler(status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		form := make(map[string]string)
		for k := range r.PostForm {
			form[k] = r.PostFormValue(k)
		}
		c.mu.Lock()
		c.reqs = append(c.reqs, capturedRequest{
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			form:        form,
		})
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	}
}

func (c *captureServer) last(t *testing.T) capturedRequest {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.reqs) == 0 {
		t.Fatal("no requests captured")
	}
	return c.reqs[len(c.reqs)-1]
}

func (c *captureServer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.reqs)
}

func storeWithToken(t *testing.T, token string) secrets.Store {
	t.Helper()
	s := secrets.NewFileStore(t.TempDir())
	if err := s.Set(context.Background(), TokenKey, token); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	return s
}

func TestTelegramSendShape(t *testing.T) {
	const token = "tok-abc"
	rec := &captureServer{}
	srv := httptest.NewServer(rec.handler(http.StatusOK, map[string]any{"ok": true}))
	defer srv.Close()

	n := NewTelegram(storeWithToken(t, token), srv.URL)
	if err := n.Send(context.Background(), "-100123", "hello world"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	req := rec.last(t)
	if want := "/bot" + token + "/sendMessage"; req.path != want {
		t.Errorf("path = %q, want %q", req.path, want)
	}
	if req.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("content-type = %q, want form-urlencoded", req.contentType)
	}
	if req.form["chat_id"] != "-100123" {
		t.Errorf("chat_id = %q, want -100123", req.form["chat_id"])
	}
	if req.form["text"] != "hello world" {
		t.Errorf("text = %q, want 'hello world'", req.form["text"])
	}
}

func TestTelegramSendAPIErrorMapped(t *testing.T) {
	rec := &captureServer{}
	srv := httptest.NewServer(rec.handler(http.StatusOK, map[string]any{"ok": false, "description": "bad request"}))
	defer srv.Close()

	n := NewTelegram(storeWithToken(t, "tok-err"), srv.URL)
	err := n.Send(context.Background(), "-100", "x")
	if err == nil {
		t.Fatal("Send on ok=false = nil, want error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("error = %q, want description carried", err)
	}
}

func TestTelegramSendUnauthorizedTokenFree(t *testing.T) {
	const token = "super-secret-bot-token-401"
	rec := &captureServer{}
	srv := httptest.NewServer(rec.handler(http.StatusUnauthorized, map[string]any{"ok": false, "description": "Unauthorized"}))
	defer srv.Close()

	n := NewTelegram(storeWithToken(t, token), srv.URL)
	err := n.Send(context.Background(), "-100", "x")
	if err == nil {
		t.Fatal("Send on 401 = nil, want error")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("token leaked into error: %v", err)
	}
	if !errors.Is(err, ErrPermanent) {
		t.Errorf("Send on 401 error = %v, want errors.Is(ErrPermanent)", err)
	}
}

func TestTelegramSend4xxIsPermanent(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden} {
		code := code
		t.Run(http.StatusText(code), func(t *testing.T) {
			t.Parallel()
			rec := &captureServer{}
			srv := httptest.NewServer(rec.handler(code, map[string]any{"ok": false, "description": "permanent"}))
			defer srv.Close()
			n := NewTelegram(storeWithToken(t, "tok-perm"), srv.URL)
			err := n.Send(context.Background(), "-100", "x")
			if !errors.Is(err, ErrPermanent) {
				t.Errorf("http %d: error = %v, want ErrPermanent wrapped", code, err)
			}
		})
	}
}

func TestTelegramSend4xxWatcherTerminatesImmediately(t *testing.T) {
	rec := &captureServer{}
	srv := httptest.NewServer(rec.handler(http.StatusUnauthorized, map[string]any{"ok": false, "description": "Unauthorized"}))
	defer srv.Close()

	dir := t.TempDir()
	mustWriteJob(t, dir, "perm-401", "-100", "hello")
	n := NewTelegram(storeWithToken(t, "tok-401-watcher"), srv.URL)
	o, _ := newObs(t)
	startWatch(t, dir, n, o)

	waitFor(t, "terminal status written on 401", func() bool {
		st, err := ReadStatus(dir, "perm-401")
		return err == nil && !st.OK
	})

	time.Sleep(100 * time.Millisecond)
	if c := rec.count(); c != 1 {
		t.Errorf("send count = %d, want 1 (permanent 401 must not retry)", c)
	}
}

func TestTelegramSendTransportErrorTokenFree(t *testing.T) {
	const token = "super-secret-bot-token-net"
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()

	n := NewTelegram(storeWithToken(t, token), srv.URL)
	err := n.Send(context.Background(), "-100", "x")
	if err == nil {
		t.Fatal("Send to closed server = nil, want error")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("token leaked into transport error: %v", err)
	}
}

func TestTelegramSendBadJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()

	n := NewTelegram(storeWithToken(t, "tok-badjson"), srv.URL)
	if err := n.Send(context.Background(), "-100", "x"); err == nil {
		t.Error("Send on malformed JSON = nil, want error")
	}
}

func TestTelegramSendTokenMissing(t *testing.T) {
	n := NewTelegram(secrets.NewFileStore(t.TempDir()), "http://127.0.0.1:0")
	err := n.Send(context.Background(), "-100", "x")
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("Send without token = %v, want ErrNotFound", err)
	}
}

func TestSendDirect(t *testing.T) {
	const token = "tok-direct"
	rec := &captureServer{}
	srv := httptest.NewServer(rec.handler(http.StatusOK, map[string]any{"ok": true}))
	defer srv.Close()

	store := storeWithToken(t, token)
	if err := SendDirect(context.Background(), store, srv.URL, "-100777", "direct hi"); err != nil {
		t.Fatalf("SendDirect: %v", err)
	}
	req := rec.last(t)
	if want := "/bot" + token + "/sendMessage"; req.path != want {
		t.Errorf("path = %q, want %q", req.path, want)
	}
	if req.form["chat_id"] != "-100777" || req.form["text"] != "direct hi" {
		t.Errorf("form = %v, want chat_id=-100777 text='direct hi'", req.form)
	}
}
