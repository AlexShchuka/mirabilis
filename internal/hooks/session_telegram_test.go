package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionTelegramContext_HappyPath(t *testing.T) {
	repoDir := t.TempDir()
	writeChatID(t, repoDir, "-100987654")

	got := sessionTelegramContext(repoDir)
	want := "telegram channel: -100987654 (cached)"
	if got != want {
		t.Errorf("sessionTelegramContext = %q, want %q", got, want)
	}
}

func TestSessionTelegramContext_NoFile_Empty(t *testing.T) {
	got := sessionTelegramContext(t.TempDir())
	if got != "" {
		t.Errorf("sessionTelegramContext with no chat-id file = %q, want empty", got)
	}
}

func TestSessionTelegramContext_EmptyFile_Empty(t *testing.T) {
	repoDir := t.TempDir()
	writeChatID(t, repoDir, "")

	got := sessionTelegramContext(repoDir)
	if got != "" {
		t.Errorf("sessionTelegramContext with empty chat-id = %q, want empty", got)
	}
}

func TestSessionStart_ChatIDPresent_InjectsContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "-100555")

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

func TestSessionStart_BothMemoryAndChatID_ConcatenatesContext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	repoDir := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repoDir)
	writeChatID(t, repoDir, "-100777")

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
	if !strings.Contains(ctx, "\n") {
		t.Errorf("context missing newline separator between memory and telegram, got:\n%s", ctx)
	}
}

func TestSessionStart_NoChatID_NoTelegramContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MIRABILIS_REPO", t.TempDir())

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
		t.Errorf("additionalContext should not contain 'telegram channel' when chat-id absent, got:\n%s", ctx)
	}
}
