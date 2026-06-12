package hooks

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
)

func writeChatID(t *testing.T, repo, id string) {
	t.Helper()
	if err := notify.WriteChatID(repo, id); err != nil {
		t.Fatalf("WriteChatID: %v", err)
	}
}

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
	t.Run("valid json", func(t *testing.T) {
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
	})
	t.Run("fallback on truncated json", func(t *testing.T) {
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
	})
}

func TestCWDBaseName(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{`{"hook_event_name":"Stop","cwd":"/workspace/myproject"}`, "myproject"},
		{`{"hook_event_name":"Stop","cwd":"/home/user/repos/other-proj"}`, "other-proj"},
		{`{"hook_event_name":"Stop","cwd":""}`, ""},
		{`{"hook_event_name":"Stop"}`, ""},
		{`not json`, ""},
		{`{}`, ""},
	}
	for _, tt := range tests {
		got := cwdBaseName([]byte(tt.give))
		if got != tt.want {
			t.Errorf("cwdBaseName(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestRepoRoot_DefaultsToWorkspace(t *testing.T) {
	t.Setenv("MIRABILIS_REPO", "")
	if got := repoRoot(); got != "/workspace" {
		t.Errorf("repoRoot() = %q, want /workspace", got)
	}
}

func TestTelegram_StopEvent_WritesQueueJob(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "chat456")

	replaceStdin(t, `{"hook_event_name":"Stop"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}

	queueDir := notify.OutboxDir(repoDir)
	jobs, err := notify.PendingJobs(queueDir)
	if err != nil {
		t.Fatalf("PendingJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("queue jobs = %d, want 1", len(jobs))
	}

	raw, err := os.ReadFile(jobs[0])
	if err != nil {
		t.Fatalf("ReadFile job: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("job file not valid JSON: %v", err)
	}
	for _, key := range []string{"id", "chat_id", "text", "created_at"} {
		if v, ok := fields[key].(string); !ok || v == "" {
			t.Errorf("job JSON field %q = %v, want non-empty string", key, fields[key])
		}
	}

	job, err := notify.ReadJob(jobs[0])
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}
	if job.ChatID != "chat456" {
		t.Errorf("job.ChatID = %q, want chat456", job.ChatID)
	}
	if !strings.Contains(job.Text, "task finished") {
		t.Errorf("job.Text = %q, want stop message", job.Text)
	}
	if job.CreatedAt.IsZero() {
		t.Error("job.CreatedAt is zero, want set")
	}
}

func TestTelegram_ChatIDFileAbsent_SilentSkip(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)

	replaceStdin(t, `{"hook_event_name":"Stop"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil when chat-id file absent", err)
	}

	jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
	if len(jobs) != 0 {
		t.Errorf("queue jobs = %d, want 0 when chat-id file absent", len(jobs))
	}
}

func TestTelegram_ChatIDFileEmpty_SilentSkip(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "")

	replaceStdin(t, `{"hook_event_name":"Stop"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil when chat-id file empty", err)
	}

	jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
	if len(jobs) != 0 {
		t.Errorf("queue jobs = %d, want 0 when chat-id file empty", len(jobs))
	}
}

func TestTelegram_IgnoresTelegramChatIDEnv(t *testing.T) {
	t.Run("env set without file writes nothing", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("MIRABILIS_REPO", repoDir)
		t.Setenv("TELEGRAM_CHAT_ID", "env-chat")

		replaceStdin(t, `{"hook_event_name":"Stop"}`)
		if err := Telegram(); err != nil {
			t.Fatalf("Telegram() = %v, want nil", err)
		}

		jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
		if len(jobs) != 0 {
			t.Errorf("queue jobs = %d, want 0 — TELEGRAM_CHAT_ID env must be ignored", len(jobs))
		}
	})
	t.Run("file wins over env", func(t *testing.T) {
		repoDir := t.TempDir()
		t.Setenv("MIRABILIS_REPO", repoDir)
		t.Setenv("TELEGRAM_CHAT_ID", "env-chat")
		writeChatID(t, repoDir, "file-chat")

		replaceStdin(t, `{"hook_event_name":"Stop"}`)
		if err := Telegram(); err != nil {
			t.Fatalf("Telegram() = %v, want nil", err)
		}

		jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
		if len(jobs) != 1 {
			t.Fatalf("queue jobs = %d, want 1", len(jobs))
		}
		job, err := notify.ReadJob(jobs[0])
		if err != nil {
			t.Fatalf("ReadJob: %v", err)
		}
		if job.ChatID != "file-chat" {
			t.Errorf("job.ChatID = %q, want file-chat (from bind-mounted file, not env)", job.ChatID)
		}
	})
}

func TestTelegram_StopEvent_NoTokenInJob(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "chat999")

	replaceStdin(t, `{"hook_event_name":"Stop"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}

	jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
	if len(jobs) == 0 {
		t.Fatal("expected a job file")
	}
	data, err := os.ReadFile(jobs[0])
	if err != nil {
		t.Fatalf("ReadFile job: %v", err)
	}
	jobContent := string(data)
	if strings.Contains(jobContent, "token") || strings.Contains(jobContent, "bot") {
		t.Errorf("job file must contain no token-like content; got: %s", jobContent)
	}
}

func TestTelegram_UnknownEvent_NoQueueJob(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "chat1")

	replaceStdin(t, `{"hook_event_name":"PreToolUse"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}

	jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
	if len(jobs) != 0 {
		t.Errorf("queue jobs = %d, want 0 for unknown event", len(jobs))
	}
}

func TestTelegram_WithCWD_AppendsProjectInJob(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "chat1")

	replaceStdin(t, `{"hook_event_name":"Stop","cwd":"/workspace/myproject"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}

	jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
	if len(jobs) == 0 {
		t.Fatal("expected a job file")
	}
	job, err := notify.ReadJob(jobs[0])
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}
	if !strings.Contains(job.Text, "[myproject]") {
		t.Errorf("job.Text = %q, want to contain [myproject]", job.Text)
	}
}

func TestTelegram_NotificationEvent_WritesQueueJob(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "chat1")

	replaceStdin(t, `{"hook_event_name":"Notification"}`)
	if err := Telegram(); err != nil {
		t.Fatalf("Telegram() = %v, want nil", err)
	}

	jobs, _ := notify.PendingJobs(notify.OutboxDir(repoDir))
	if len(jobs) != 1 {
		t.Errorf("queue jobs = %d, want 1 for Notification", len(jobs))
	}
}

func TestDispatchUnknown(t *testing.T) {
	err := Dispatch("nonexistent")
	if err == nil {
		t.Error("Dispatch(unknown) should return error")
	}
}

func TestDispatchTelegram(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	replaceStdin(t, `{"hook_event_name":"Stop"}`)
	if err := Dispatch("telegram"); err != nil {
		t.Errorf("Dispatch(telegram) with no chat-id file = %v, want nil", err)
	}
}
