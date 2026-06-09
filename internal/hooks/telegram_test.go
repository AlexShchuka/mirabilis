package hooks

import (
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
