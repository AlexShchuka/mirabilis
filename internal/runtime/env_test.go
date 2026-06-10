package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeychainEnv(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"telegram-token", "TELEGRAM_BOT_TOKEN"},
		{"telegram-chat", "TELEGRAM_CHAT_ID"},
		{"unknown-name", ""},
	}
	for _, tt := range tests {
		got := keychainEnv(tt.name)
		if got != tt.want {
			t.Errorf("keychainEnv(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestKeychainGet_UnknownName(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	got := keychainGet("unknown-name")
	if got != "" {
		t.Errorf("keychainGet(unknown-name) = %q, want empty", got)
	}
}

func TestKeychainGet_AccountOverride(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "myaccount")
	t.Setenv("TELEGRAM_BOT_TOKEN", "overridden")
	got := keychainGet("telegram-token")
	if got != "overridden" {
		t.Errorf("keychainGet with account override = %q, want overridden", got)
	}
}

func TestKeychainGet_NonDarwinEnvFallback(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok123")
	t.Setenv("TELEGRAM_CHAT_ID", "chat456")

	got := keychainGet("telegram-token")
	if got != "tok123" {
		t.Errorf("keychainGet(telegram-token) = %q, want tok123 (non-darwin env fallback)", got)
	}

	got2 := keychainGet("telegram-chat")
	if got2 != "chat456" {
		t.Errorf("keychainGet(telegram-chat) = %q, want chat456", got2)
	}
}

func TestGitShort(t *testing.T) {
	dir := makeGitRepo(t)
	sha := GitShort(dir)
	if sha == "unknown" {
		t.Fatal("GitShort returned 'unknown' for a valid git repo")
	}
	if len(sha) < 4 {
		t.Errorf("GitShort = %q, expected a short sha (>=4 chars)", sha)
	}

	nonRepo := t.TempDir()
	got := GitShort(nonRepo)
	if got != "unknown" {
		t.Errorf("GitShort(non-repo) = %q, want 'unknown'", got)
	}
}

func TestComposeEnv_EmptyManagedValuesOmitted(t *testing.T) {
	repo := makeGitRepo(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	env := ComposeEnv(repo)
	for _, kv := range env {
		if kv == "TELEGRAM_BOT_TOKEN=" || kv == "TELEGRAM_CHAT_ID=" {
			t.Errorf("empty managed key must not appear in env: %q", kv)
		}
	}
}

func TestComposeEnv_ManagedKeys(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("STACKS=go,rust\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok-test")
	t.Setenv("TELEGRAM_CHAT_ID", "chat-test")
	t.Setenv("MIRABILIS_VERSION", "old-version")

	env := ComposeEnv(repo)

	counts := map[string]int{}
	values := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		counts[k]++
		values[k] = v
	}

	for _, key := range []string{"MIRABILIS_VERSION", "TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"} {
		if counts[key] > 1 {
			t.Errorf("managed key %s appears %d times, want exactly once", key, counts[key])
		}
	}

	sha := GitShort(repo)
	if values["MIRABILIS_VERSION"] != sha {
		t.Errorf("MIRABILIS_VERSION = %q, want %q", values["MIRABILIS_VERSION"], sha)
	}

	if v := values["STACKS"]; v != "go,rust" {
		t.Errorf("STACKS = %q, want go,rust", v)
	}
}
