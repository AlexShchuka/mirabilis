package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stdinClosed(t *testing.T) {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	pw.Close()
	pr.Close()
	old := os.Stdin
	os.Stdin = pr
	t.Cleanup(func() { os.Stdin = old })
}

func TestTelegram_StdinReadError_NoError(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	t.Setenv("TELEGRAM_CHAT_ID", "chat")
	stdinClosed(t)
	if err := Telegram(); err != nil {
		t.Errorf("Telegram = %v, want nil when stdin read fails", err)
	}
}

func TestMemoryIndex_UnreadableFileSkipped(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("an unreadable file is still readable as root")
	}
	dir := t.TempDir()
	writeTestFile(t, dir, "about-me.md", `---
category: about-me
memory_type: semantic
summary: ok
---

# About Me

- INVARIANT one
`)
	bad := filepath.Join(dir, "dev-principles.md")
	if err := os.WriteFile(bad, []byte("---\ncategory: dev-principles\n---\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })

	idx, err := memoryIndex(dir)
	if err != nil {
		t.Fatalf("memoryIndex = %v, want nil", err)
	}
	if !strings.Contains(idx, "about-me") {
		t.Errorf("index missing readable category, got:\n%s", idx)
	}
	if strings.Contains(idx, "dev-principles") {
		t.Errorf("unreadable file should be skipped, but appeared in:\n%s", idx)
	}
}

func TestParseFrontmatter_PreambleBeforeDelimiter(t *testing.T) {
	data := []byte("garbage line\nanother\n---\ncategory: about-me\nmemory_type: semantic\nsummary: s\n---\n")
	meta := parseFrontmatter(data)
	if meta.category != "about-me" {
		t.Errorf("category = %q, want about-me — lines before the opening --- must be ignored", meta.category)
	}
}

func TestSessionStart_HomeUnsetTakesFallbackBranch(t *testing.T) {
	t.Setenv("HOME", "")
	replaceStdin(t, "")
	getOut := captureStdout(t)
	err := SessionStart()
	out := getOut()
	if err != nil {
		t.Fatalf("SessionStart = %v, want nil when HOME is unset", err)
	}
	var payload map[string]any
	if uerr := json.Unmarshal([]byte(out), &payload); uerr != nil {
		t.Fatalf("output not valid JSON: %v\n%s", uerr, out)
	}
	hookOut, ok := payload["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("payload missing hookSpecificOutput: %s", out)
	}
	if name, _ := hookOut["hookEventName"].(string); name != "SessionStart" {
		t.Errorf("hookEventName = %q, want SessionStart", name)
	}
}

func TestSessionStart_WriteMemoryWarns_StillEmitsContext(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based write failure is not reproducible as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	memDir := filepath.Join(tmp, ".claude", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, memDir, "about-me.md", `---
category: about-me
memory_type: semantic
summary: s
---

# About Me

- INVARIANT one
`)
	if err := os.Chmod(memDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(memDir, 0o755) })

	replaceStdin(t, "")
	getOut := captureStdout(t)
	getErr := captureStderr(t)
	err := SessionStart()
	out := getOut()
	errOut := getErr()
	if err != nil {
		t.Fatalf("SessionStart = %v, want nil even when MEMORY.md write fails", err)
	}
	if !strings.Contains(errOut, "write MEMORY.md") {
		t.Errorf("stderr = %q, want WARN naming write MEMORY.md", errOut)
	}
	if !strings.Contains(out, "about-me") {
		t.Errorf("context still expected on stdout despite write failure; got:\n%s", out)
	}
}

func captureStderr(t *testing.T) (restore func() string) {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = pw
	t.Cleanup(func() {
		pw.Close()
		os.Stderr = old
	})
	return func() string {
		pw.Close()
		os.Stderr = old
		var buf [65536]byte
		n, _ := pr.Read(buf[:])
		pr.Close()
		return string(buf[:n])
	}
}

func captureStdout(t *testing.T) (restore func() string) {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = pw
	t.Cleanup(func() {
		pw.Close()
		os.Stdout = old
	})
	return func() string {
		pw.Close()
		os.Stdout = old
		var buf [65536]byte
		n, _ := pr.Read(buf[:])
		pr.Close()
		return string(buf[:n])
	}
}

func replaceStdin(t *testing.T, data string) {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = pr
	t.Cleanup(func() {
		os.Stdin = old
		pr.Close()
	})
	if _, err := pw.WriteString(data); err != nil {
		t.Fatal(err)
	}
	pw.Close()
}
