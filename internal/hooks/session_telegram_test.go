package hooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// ── helpers ────────────────────────────────────────────────────────────────

func makeTokenPath(t *testing.T, token string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "token")
	if err := os.WriteFile(p, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func makeCachePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".mirabilis-telegram-channel")
}

func makeUpdatesServer(t *testing.T, chatID int64, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			calls.Add(1)
		}
		var result any
		if chatID != 0 {
			result = map[string]any{
				"ok": true,
				"result": []any{
					map[string]any{
						"channel_post": map[string]any{
							"chat": map[string]any{"id": chatID},
						},
					},
				},
			}
		} else {
			result = map[string]any{"ok": true, "result": []any{}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	return srv
}

// ── detectAndCacheTelegramChannel ──────────────────────────────────────────

func TestDetectAndCacheTelegramChannel_HappyPath(t *testing.T) {
	var calls atomic.Int32
	srv := makeUpdatesServer(t, -100123456, &calls)
	defer srv.Close()

	tokenPath := makeTokenPath(t, "bot123:token")
	cachePath := makeCachePath(t)

	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)

	if calls.Load() == 0 {
		t.Error("expected HTTP call; made 0")
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
	got := strings.TrimRight(string(data), "\r\n")
	if got != "-100123456" {
		t.Errorf("cached channel = %q, want -100123456", got)
	}
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("cache file mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestDetectAndCacheTelegramChannel_NoTokenFile_NoHTTPCall(t *testing.T) {
	var calls atomic.Int32
	srv := makeUpdatesServer(t, -100123, &calls)
	defer srv.Close()

	cachePath := makeCachePath(t)
	detectAndCacheTelegramChannel("/nonexistent/token/file", cachePath, srv.URL)

	if calls.Load() != 0 {
		t.Errorf("HTTP calls = %d, want 0 when token file absent", calls.Load())
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be written when token absent")
	}
}

func TestDetectAndCacheTelegramChannel_EmptyToken_NoHTTPCall(t *testing.T) {
	var calls atomic.Int32
	srv := makeUpdatesServer(t, -100123, &calls)
	defer srv.Close()

	tokenPath := makeTokenPath(t, "  \n")
	// Override the written file with whitespace-only content.
	if err := os.WriteFile(tokenPath, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cachePath := makeCachePath(t)
	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)

	if calls.Load() != 0 {
		t.Errorf("HTTP calls = %d, want 0 when token empty", calls.Load())
	}
}

func TestDetectAndCacheTelegramChannel_NoChannelPost_NoCacheFile(t *testing.T) {
	var calls atomic.Int32
	srv := makeUpdatesServer(t, 0 /* no chat_id */, &calls)
	defer srv.Close()

	tokenPath := makeTokenPath(t, "bot123:token")
	cachePath := makeCachePath(t)

	getErr := captureStderr(t)
	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)
	errOut := getErr()

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be written when no channel_post found")
	}
	if !strings.Contains(errOut, "post anything in the channel") {
		t.Errorf("stderr = %q, want hint about posting in channel", errOut)
	}
}

func TestDetectAndCacheTelegramChannel_APIError_NoCacheFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "Forbidden"})
	}))
	defer srv.Close()

	tokenPath := makeTokenPath(t, "bot123:token")
	cachePath := makeCachePath(t)

	getErr := captureStderr(t)
	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)
	errOut := getErr()

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be written when API returns ok=false")
	}
	_ = errOut // fail-soft may log or not
}

func TestDetectAndCacheTelegramChannel_HTTPError_NoCache(t *testing.T) {
	// Use a URL that will refuse connections immediately.
	tokenPath := makeTokenPath(t, "bot123:token")
	cachePath := makeCachePath(t)

	getErr := captureStderr(t)
	detectAndCacheTelegramChannel(tokenPath, cachePath, "http://127.0.0.1:1")
	errOut := getErr()

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be written when HTTP fails")
	}
	if !strings.Contains(errOut, "channel not detected yet") {
		t.Errorf("stderr = %q, want 'channel not detected yet'", errOut)
	}
}

func TestDetectAndCacheTelegramChannel_InvalidJSON_NoCacheFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	tokenPath := makeTokenPath(t, "bot123:token")
	cachePath := makeCachePath(t)

	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be written when response JSON is invalid")
	}
}

func TestDetectAndCacheTelegramChannel_ChannelPostNilEntry_NoCacheFile(t *testing.T) {
	// Result has entries but channel_post is null/absent.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": []any{map[string]any{"message": "ignored"}},
		})
	}))
	defer srv.Close()

	tokenPath := makeTokenPath(t, "bot123:token")
	cachePath := makeCachePath(t)

	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be written when channel_post is absent")
	}
}

func TestDetectAndCacheTelegramChannel_CacheWriteFails_LogsWarn(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	var calls atomic.Int32
	srv := makeUpdatesServer(t, -100123456, &calls)
	defer srv.Close()

	tokenPath := makeTokenPath(t, "bot123:token")

	// Make the cache path's parent directory read-only so WriteFile fails.
	roDir := t.TempDir()
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })
	cachePath := filepath.Join(roDir, ".mirabilis-telegram-channel")

	getErr := captureStderr(t)
	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)
	errOut := getErr()

	if !strings.Contains(errOut, "write telegram channel cache") {
		t.Errorf("stderr = %q, want 'write telegram channel cache' WARN", errOut)
	}
}

func TestDetectAndCacheTelegramChannel_ZeroChatID_NoCacheFile(t *testing.T) {
	// A channel_post with chat.id == 0 must be ignored (continue).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []any{
				map[string]any{
					"channel_post": map[string]any{
						"chat": map[string]any{"id": 0},
					},
				},
			},
		})
	}))
	defer srv.Close()

	tokenPath := makeTokenPath(t, "bot123:token")
	cachePath := makeCachePath(t)

	getErr := captureStderr(t)
	detectAndCacheTelegramChannel(tokenPath, cachePath, srv.URL)
	_ = getErr()

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be written when chat_id is 0")
	}
}

// ── sessionTelegramContext ─────────────────────────────────────────────────

func TestSessionTelegramContext_HappyPath(t *testing.T) {
	cachePath := makeCachePath(t)
	if err := os.WriteFile(cachePath, []byte("-100987654\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := sessionTelegramContext(cachePath)
	want := "telegram channel: -100987654 (cached)"
	if got != want {
		t.Errorf("sessionTelegramContext = %q, want %q", got, want)
	}
}

func TestSessionTelegramContext_NoFile_Empty(t *testing.T) {
	got := sessionTelegramContext("/nonexistent/cache")
	if got != "" {
		t.Errorf("sessionTelegramContext with no file = %q, want empty", got)
	}
}

func TestSessionTelegramContext_EmptyFile_Empty(t *testing.T) {
	cachePath := makeCachePath(t)
	if err := os.WriteFile(cachePath, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := sessionTelegramContext(cachePath)
	if got != "" {
		t.Errorf("sessionTelegramContext with empty cache = %q, want empty", got)
	}
}

// ── SessionStart with Telegram context injection ───────────────────────────

func TestSessionStart_TelegramChannelCached_InjectsContext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Pre-populate the channel cache.
	cd := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-telegram-channel"), []byte("-100555\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	replaceStdin(t, "")
	getOut := captureStdout(t)
	if err := SessionStart(); err != nil {
		_ = getOut()
		t.Fatalf("SessionStart: %v", err)
	}
	out := getOut()

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	hookOut, _ := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hookOut["additionalContext"].(string)
	if !strings.Contains(ctx, "telegram channel") {
		t.Errorf("additionalContext missing 'telegram channel', got:\n%s", ctx)
	}
	if !strings.Contains(ctx, "-100555") {
		t.Errorf("additionalContext missing channel id '-100555', got:\n%s", ctx)
	}
}

func TestSessionStart_BothMemoryAndTelegramChannel_ConcatenatesContext(t *testing.T) {
	// Covers the additionalContext += "\n" + tgCtx branch (both memory AND channel cached).
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create memory files so idx is non-empty.
	memDir := filepath.Join(tmp, ".claude", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, memDir, "about-me.md", `---
category: about-me
memory_type: semantic
summary: I am a developer.
max_lines: 80
---

# About Me

- fact one
`)

	// Pre-populate the channel cache.
	cd := filepath.Join(tmp, ".claude")
	if err := os.WriteFile(filepath.Join(cd, ".mirabilis-telegram-channel"), []byte("-100777\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	replaceStdin(t, "")
	getOut := captureStdout(t)
	if err := SessionStart(); err != nil {
		_ = getOut()
		t.Fatalf("SessionStart: %v", err)
	}
	out := getOut()

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	hookOut, _ := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hookOut["additionalContext"].(string)
	if !strings.Contains(ctx, "about-me") {
		t.Errorf("context missing 'about-me', got:\n%s", ctx)
	}
	if !strings.Contains(ctx, "telegram channel") {
		t.Errorf("context missing 'telegram channel', got:\n%s", ctx)
	}
	// The two parts should be joined with a newline.
	if !strings.Contains(ctx, "\n") {
		t.Errorf("context missing newline separator between memory and telegram, got:\n%s", ctx)
	}
}

func TestSessionStart_TelegramChannelNotCached_NoContext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// No channel cache file; also no token file (no /run/secrets/ here).

	replaceStdin(t, "")
	getOut := captureStdout(t)
	if err := SessionStart(); err != nil {
		_ = getOut()
		t.Fatalf("SessionStart: %v", err)
	}
	out := getOut()

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	hookOut, _ := payload["hookSpecificOutput"].(map[string]any)
	ctx, _ := hookOut["additionalContext"].(string)
	if strings.Contains(ctx, "telegram channel") {
		t.Errorf("additionalContext should not contain 'telegram channel' when cache absent, got:\n%s", ctx)
	}
}
