package hooks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMessageFor(t *testing.T) {
	tests := []struct {
		event   string
		wantMsg string
		wantOK  bool
	}{
		{"Notification", "❓ mirabilis: needs your input", true},
		{"Stop", "✅ mirabilis: task finished", true},
		{"PreToolUse", "", false},
		{"", "", false},
		{"unknown", "", false},
	}
	for _, tt := range tests {
		msg, ok := messageFor(tt.event)
		if ok != tt.wantOK {
			t.Errorf("messageFor(%q) ok = %v, want %v", tt.event, ok, tt.wantOK)
		}
		if msg != tt.wantMsg {
			t.Errorf("messageFor(%q) = %q, want %q", tt.event, msg, tt.wantMsg)
		}
	}
}

func TestEventName(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{`{"hook_event_name":"Notification","session_id":"x"}`, "Notification"},
		{`{"hook_event_name":"Stop"}`, "Stop"},
		{`{"hook_event_name":"PreToolUse"}`, "PreToolUse"},
		{`{"other":"field"}`, ""},
		{`not json at all`, ""},
		{``, ""},
		{`{"hook_event_name":"Notification","extra":1}`, "Notification"},
		{`{"hook_event_name" : "Stop" }`, "Stop"},
	}
	for _, tt := range tests {
		got := eventName([]byte(tt.give))
		if got != tt.want {
			t.Errorf("eventName(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestEventNameFallback(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{`{"hook_event_name":"Notification","broken`, "Notification"},
		{`{"hook_event_name":"Stop","broken`, "Stop"},
		{`{"msg":"please send Notification","hook_event_name":"Stop","broken`, "Stop"},
		{`{"hook_event_name":"Stop"`, "Stop"},
		{`{"msg":"please send Notification"}`, ""},
	}
	for _, tt := range tests {
		got := eventName([]byte(tt.give))
		if got != tt.want {
			t.Errorf("eventName(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestTelegramNoOpWhenEnvEmpty(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if err := Telegram(); err != nil {
		t.Errorf("Telegram() = %v, want nil when env empty", err)
	}
}

func TestDispatchUnknown(t *testing.T) {
	err := Dispatch("nonexistent")
	if err == nil {
		t.Error("Dispatch(unknown) should return error")
	}
}

func TestDispatchTelegram(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if err := Dispatch("telegram"); err != nil {
		t.Errorf("Dispatch(telegram) with empty env = %v, want nil", err)
	}
}

func setupTelegramServer(t *testing.T, token, chat string, counter *atomic.Int32, assertFn func(r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		if assertFn != nil {
			assertFn(r)
		}
		w.WriteHeader(http.StatusOK)
	}))
	old := telegramAPI
	telegramAPI = srv.URL
	t.Cleanup(func() {
		telegramAPI = old
		srv.Close()
	})
	t.Setenv("TELEGRAM_BOT_TOKEN", token)
	t.Setenv("TELEGRAM_CHAT_ID", chat)
	return srv
}

func TestTelegram_StopEvent_SendsMessage(t *testing.T) {
	var counter atomic.Int32
	var gotPath, gotChatID, gotText string
	setupTelegramServer(t, "tok123", "chat456", &counter, func(r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		gotChatID = r.FormValue("chat_id")
		gotText = r.FormValue("text")
	})
	replaceStdin(t, `{"hook_event_name":"Stop"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}
	if counter.Load() != 1 {
		t.Errorf("HTTP calls = %d, want 1", counter.Load())
	}
	if !strings.Contains(gotPath, "tok123/sendMessage") {
		t.Errorf("path = %q, want to contain tok123/sendMessage", gotPath)
	}
	if gotChatID != "chat456" {
		t.Errorf("chat_id = %q, want chat456", gotChatID)
	}
	if gotText != "✅ mirabilis: task finished" {
		t.Errorf("text = %q, want stop message", gotText)
	}
}

func TestTelegram_NotificationEvent_SendsMessage(t *testing.T) {
	var counter atomic.Int32
	setupTelegramServer(t, "tok1", "chat1", &counter, nil)
	replaceStdin(t, `{"hook_event_name":"Notification"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}
	if counter.Load() != 1 {
		t.Errorf("HTTP calls = %d, want 1", counter.Load())
	}
}

func TestTelegram_UnknownEvent_NoHTTPCall(t *testing.T) {
	var counter atomic.Int32
	setupTelegramServer(t, "tok1", "chat1", &counter, nil)
	replaceStdin(t, `{"hook_event_name":"PreToolUse"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}
	if counter.Load() != 0 {
		t.Errorf("HTTP calls = %d, want 0 for unknown event", counter.Load())
	}
}

func TestTelegram_ServerUnreachable_NoError(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok1")
	t.Setenv("TELEGRAM_CHAT_ID", "chat1")
	old := telegramAPI
	telegramAPI = "http://127.0.0.1:0"
	t.Cleanup(func() { telegramAPI = old })
	replaceStdin(t, `{"hook_event_name":"Stop"}`)
	if err := Telegram(); err != nil {
		t.Errorf("Telegram() = %v, want nil even on unreachable server", err)
	}
}
