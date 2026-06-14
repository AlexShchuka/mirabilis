package steps

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func newTelegramForTest(t *testing.T, store *fakeStore) *telegramStep {
	t.Helper()
	return newTelegram(newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), store))
}

func TestTelegramCheck(t *testing.T) {
	t.Parallel()
	t.Run("unconfigured", func(t *testing.T) {
		t.Parallel()
		mustCheck(t, newTelegramForTest(t, newFakeStore()), false)
	})
	t.Run("skip persisted", func(t *testing.T) {
		t.Parallel()
		s := newTelegramForTest(t, newFakeStore())
		if err := dotenvWrite(s.d.Repo, telegramEnvKey, telegramSkip); err != nil {
			t.Fatal(err)
		}
		mustCheck(t, s, true)
	})
	t.Run("token without chat id", func(t *testing.T) {
		t.Parallel()
		store := newFakeStore()
		store.m[notify.TokenKey] = "12345:token"
		mustCheck(t, newTelegramForTest(t, store), false)
	})
	t.Run("token and chat id", func(t *testing.T) {
		t.Parallel()
		store := newFakeStore()
		store.m[notify.TokenKey] = "12345:token"
		s := newTelegramForTest(t, store)
		if err := notify.WriteChatID(s.d.Repo, "-10042"); err != nil {
			t.Fatal(err)
		}
		mustCheck(t, s, true)
	})
}

func TestTelegramRunSkipPersists(t *testing.T) {
	t.Parallel()
	s := newTelegramForTest(t, newFakeStore())
	evs, err := runStep(t, s, func(any) pipeline.Result { return pipeline.Result{Value: "skip"} })
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, ok := waitingEvent(t, evs).Payload.(TelegramSetup); !ok {
		t.Fatalf("payload = %T, want TelegramSetup", waitingEvent(t, evs).Payload)
	}
	if v, ok := dotenvRead(s.d.Repo, telegramEnvKey); !ok || v != telegramSkip {
		t.Fatalf("TELEGRAM = %q ok=%v, want skip", v, ok)
	}
	mustCheck(t, s, true)
}

func TestTelegramRunStoresTokenAndDetectsChat(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/bot12345:token/getUpdates") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"channel_post":{"chat":{"id":-100777,"type":"channel"}}}]}`))
	}))
	defer srv.Close()
	store := newFakeStore()
	s := newTelegramForTest(t, store)
	s.baseURL = srv.URL
	evs, err := runStep(t, s, func(any) pipeline.Result { return pipeline.Result{Value: "12345:token"} })
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := store.m[notify.TokenKey]; got != "12345:token" {
		t.Fatalf("stored token = %q", got)
	}
	if id, err := notify.ReadChatID(s.d.Repo); err != nil || id != "-100777" {
		t.Fatalf("chat id = %q err=%v", id, err)
	}
	mustCheck(t, s, true)

	var lines []string
	for _, ev := range evs {
		if ev.Kind == pipeline.EvLine && ev.Line != "" {
			lines = append(lines, ev.Line)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("got %d EvLine events with non-empty Line, want at least 2 (saving token + detecting channel)", len(lines))
	}
	if lines[0] != "saving token…" {
		t.Errorf("lines[0] = %q, want \"saving token…\"", lines[0])
	}
	if lines[1] != "detecting channel…" {
		t.Errorf("lines[1] = %q, want \"detecting channel…\"", lines[1])
	}
}

func TestTelegramRunCancelled(t *testing.T) {
	t.Parallel()
	s := newTelegramForTest(t, newFakeStore())
	_, err := runStep(t, s, func(any) pipeline.Result { return pipeline.Result{Cancelled: true} })
	if !errors.Is(err, pipeline.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	mustCheck(t, s, false)
}
