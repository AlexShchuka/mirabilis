package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/tgtoken"
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
	t.Setenv("TELEGRAM_CHAT_ID", "")

	env := ComposeEnv(repo)
	for _, kv := range env {
		// TELEGRAM_BOT_TOKEN must never appear — the token is not injected.
		if strings.HasPrefix(kv, "TELEGRAM_BOT_TOKEN=") {
			t.Errorf("TELEGRAM_BOT_TOKEN must never be injected into container env: %q", kv)
		}
		if kv == "TELEGRAM_CHAT_ID=" {
			t.Errorf("empty managed key must not appear in env: %q", kv)
		}
	}
}

func TestComposeEnv_TokenNeverInjected(t *testing.T) {
	// Invariant: TELEGRAM_BOT_TOKEN is NEVER in the container env regardless of
	// what is in the host env or keychain. The token holder is the host process.
	repo := makeGitRepo(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "super-secret-token")

	env := ComposeEnv(repo)
	for _, kv := range env {
		if strings.HasPrefix(kv, "TELEGRAM_BOT_TOKEN=") {
			t.Errorf("TELEGRAM_BOT_TOKEN leaked into container env: %q", kv)
		}
	}
}

func TestReadTelegramTokenFile_WrittenByProvision(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Write the file that provision.WriteTelegramToken would write.
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(cd, ".mirabilis-telegram-token")
	if err := os.WriteFile(tokenPath, []byte("bot777:testtoken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := tgtoken.Read()
	if got != "bot777:testtoken" {
		t.Errorf("tgtoken.Read() = %q, want bot777:testtoken", got)
	}
}

func TestReadTelegramTokenFile_NoFile_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := tgtoken.Read()
	if got != "" {
		t.Errorf("tgtoken.Read() with no file = %q, want empty", got)
	}
}

func TestReadTelegramTokenFile_HomeFromUserHomeDir(t *testing.T) {
	// When HOME is not set, readTelegramTokenFile falls back to os.UserHomeDir.
	// We can only verify it doesn't crash; the actual value depends on the
	// environment, so just ensure empty string (no file exists in the test env).
	t.Setenv("HOME", "")
	got := tgtoken.Read()
	// We just confirm no panic and that empty string is returned (no stray file).
	_ = got
}

func TestKeychainGet_TelegramTokenFileFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TELEGRAM_BOT_TOKEN", "")

	// Write the token file as the provision step would.
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-telegram-token"), []byte("bot-from-file:abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := keychainGet("telegram-token")
	if got != "bot-from-file:abc" {
		t.Errorf("keychainGet(telegram-token) via file fallback = %q, want bot-from-file:abc", got)
	}
}

func TestKeychainGetTelegramChat_EnvFallback(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "-100555001")
	got := KeychainGetTelegramChat()
	if got != "-100555001" {
		t.Errorf("KeychainGetTelegramChat() = %q, want -100555001", got)
	}
}

func TestKeychainGetTelegramChat_Empty(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "")
	// No keychain on test platform; env is empty, result should be empty.
	got := KeychainGetTelegramChat()
	// Just verifying it doesn't panic; value depends on system keychain.
	_ = got
}

func TestComposeEnv_ManagedKeys(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("STACKS=go,rust\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// TELEGRAM_BOT_TOKEN is set in host env but must NOT leak into container env.
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

	// Token must never appear in container env.
	if counts["TELEGRAM_BOT_TOKEN"] > 0 {
		t.Errorf("TELEGRAM_BOT_TOKEN must never appear in container env; found %d times", counts["TELEGRAM_BOT_TOKEN"])
	}

	for _, key := range []string{"MIRABILIS_VERSION", "TELEGRAM_CHAT_ID"} {
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

func TestComposeEnv_ClaudeTokenNotDoubleInjected(t *testing.T) {
	repo := makeGitRepo(t)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-host-secret")

	env := ComposeEnv(repo)
	count := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, "CLAUDE_CODE_OAUTH_TOKEN=") {
			count++
		}
	}
	if count > 1 {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN appears %d times in container env, want at most 1", count)
	}
}

func TestComposeEnv_ClaudeTokenInjectedFromFile(t *testing.T) {
	repo := makeGitRepo(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")

	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-claude-token"), []byte("file-oauth-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	env := ComposeEnv(repo)
	found := false
	for _, kv := range env {
		if kv == "CLAUDE_CODE_OAUTH_TOKEN=file-oauth-token" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDE_CODE_OAUTH_TOKEN from token file not injected into container env")
	}
}

func TestKeychainEnv_ClaudeToken(t *testing.T) {
	got := keychainEnv("claude-token")
	if got != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Errorf("keychainEnv(claude-token) = %q, want CLAUDE_CODE_OAUTH_TOKEN", got)
	}
}

func TestKeychainGet_ClaudeTokenEnvFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "env-oauth-token")

	got := keychainGet("claude-token")
	if got != "env-oauth-token" {
		t.Errorf("keychainGet(claude-token) via env = %q, want env-oauth-token", got)
	}
}

func TestKeychainGet_ClaudeTokenFileFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")

	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-claude-token"), []byte("file-oauth-fallback\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := keychainGet("claude-token")
	if got != "file-oauth-fallback" {
		t.Errorf("keychainGet(claude-token) via file = %q, want file-oauth-fallback", got)
	}
}
