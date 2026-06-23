package hooks

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision"
)

var bashPrefix = []string{"bash", "-lc"}

func setRunner(t *testing.T, r exec.Runner) {
	t.Helper()
	old := runner
	runner = r
	t.Cleanup(func() { runner = old })
}

func writeUpstreamFile(t *testing.T, home, upstream string) {
	t.Helper()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, provision.UpstreamFileName), []byte(upstream+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scriptOf(t *testing.T, call exec.FakeCall) string {
	t.Helper()
	if len(call.Argv) != 3 || call.Argv[0] != "bash" || call.Argv[1] != "-lc" {
		t.Fatalf("call argv = %v, want [bash -lc <script>]", call.Argv)
	}
	return call.Argv[2]
}

func TestEnsureProxyForSession_UpstreamFileAbsent_ZeroRunnerCalls(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fake := exec.NewFake()
	setRunner(t, fake)

	ensureProxyForSession(context.Background())

	if calls := fake.Calls(); len(calls) != 0 {
		t.Errorf("runner calls = %d (%v), want 0 when upstream file absent", len(calls), calls)
	}
}

func TestEnsureProxyForSession_ProxyReachable_ProbeOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUpstreamFile(t, home, "http://host.docker.internal:8788")

	fake := exec.NewFake().Expect(bashPrefix, "", nil)
	setRunner(t, fake)

	ensureProxyForSession(context.Background())

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("runner calls = %d, want 1 (probe only)", len(calls))
	}
	probe := scriptOf(t, calls[0])
	if !strings.Contains(probe, "curl -fsS http://127.0.0.1:8787/stats") {
		t.Errorf("probe script = %q, want curl against :8787/stats", probe)
	}
	if strings.Contains(probe, "setsid") {
		t.Errorf("probe script = %q, must not start headroom", probe)
	}
}

func TestEnsureProxyForSession_ProxyUnreachable_StartsHeadroomAndPolls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUpstreamFile(t, home, "http://host.docker.internal:8788")

	fake := exec.NewFake().
		Expect(bashPrefix, "", errors.New("connection refused")).
		Expect(bashPrefix, "", nil).
		Expect(bashPrefix, "", nil)
	setRunner(t, fake)

	getErr := captureStderr(t)
	ensureProxyForSession(context.Background())
	errOut := getErr()

	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("runner calls = %d, want 3 (probe, start, poll)", len(calls))
	}

	start := scriptOf(t, calls[1])
	if strings.Contains(start, "ANTHROPIC_TARGET_API_URL") {
		t.Errorf("start script = %q, ANTHROPIC_TARGET_API_URL must not appear in shell script (CWE-78 fix)", start)
	}
	if !strings.Contains(start, "setsid nohup") {
		t.Errorf("start script = %q, want setsid nohup", start)
	}
	if !strings.Contains(start, filepath.Join(home, ".headroom-venv/bin/headroom")) {
		t.Errorf("start script = %q, want headroom venv bin path", start)
	}
	if !strings.Contains(start, `proxy --mode "cache" >"$HOME/.headroom-proxy.log" 2>&1 &`) {
		t.Errorf("start script = %q, want --mode \"cache\" and log redirect", start)
	}
	startEnv := calls[1].Env
	found := false
	for _, e := range startEnv {
		if e == "ANTHROPIC_TARGET_API_URL=http://host.docker.internal:8788" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("start call Env = %v, want ANTHROPIC_TARGET_API_URL=http://host.docker.internal:8788 passed via process env", startEnv)
	}

	poll := scriptOf(t, calls[2])
	if !strings.Contains(poll, "seq 1 60") {
		t.Errorf("poll script = %q, want 60s poll loop", poll)
	}
	if !strings.Contains(poll, "curl -fsS http://127.0.0.1:8787/stats") {
		t.Errorf("poll script = %q, want curl against :8787/stats", poll)
	}

	if errOut != "" {
		t.Errorf("stderr = %q, want empty on successful start", errOut)
	}
}

func TestEnsureProxyForSession_EmptyUpstream_StartsWithoutEnvPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUpstreamFile(t, home, "")

	fake := exec.NewFake().
		Expect(bashPrefix, "", errors.New("connection refused")).
		Expect(bashPrefix, "", nil).
		Expect(bashPrefix, "", nil)
	setRunner(t, fake)

	ensureProxyForSession(context.Background())

	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("runner calls = %d, want 3", len(calls))
	}
	start := scriptOf(t, calls[1])
	if strings.Contains(start, "ANTHROPIC_TARGET_API_URL") {
		t.Errorf("start script = %q, want no env prefix for empty upstream", start)
	}
	if !strings.HasPrefix(start, "setsid nohup") {
		t.Errorf("start script = %q, want to begin with setsid nohup", start)
	}
	if len(calls[1].Env) != 0 {
		t.Errorf("start call Env = %v, want empty when upstream is empty", calls[1].Env)
	}
}

func TestEnsureProxyForSession_PollFails_WarnsOnStderr(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUpstreamFile(t, home, "http://host.docker.internal:8788")

	fake := exec.NewFake().
		Expect(bashPrefix, "", errors.New("connection refused")).
		Expect(bashPrefix, "", nil).
		Expect(bashPrefix, "", errors.New("exit status 1"))
	setRunner(t, fake)

	getErr := captureStderr(t)
	ensureProxyForSession(context.Background())
	errOut := getErr()

	if !strings.Contains(errOut, "headroom proxy not ready") {
		t.Errorf("stderr = %q, want WARN about proxy not ready", errOut)
	}
}

func TestEnsureProxyForSession_NeverWritesSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeUpstreamFile(t, home, "http://host.docker.internal:8788")

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	original := []byte("{\n  \"env\": {\n    \"ANTHROPIC_BASE_URL\": \"http://127.0.0.1:8787\"\n  },\n  \"theme\": \"dark\"\n}\n")
	if err := os.WriteFile(settingsPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	fake := exec.NewFake().
		Expect(bashPrefix, "", errors.New("connection refused")).
		Expect(bashPrefix, "", nil).
		Expect(bashPrefix, "", nil)
	setRunner(t, fake)

	ensureProxyForSession(context.Background())

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Errorf("settings.json changed:\nbefore: %s\nafter:  %s", original, after)
	}
}

func TestSessionStart_UpstreamPresent_ProbesViaRunner(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MIRABILIS_REPO", t.TempDir())
	writeUpstreamFile(t, home, "http://host.docker.internal:8788")

	ghCheck := []string{"git", "config", "--get-all", "credential.https://github.com.helper"}
	fake := exec.NewFake().
		Expect(bashPrefix, "", nil).
		Expect(ghCheck, "\n!/usr/bin/gh auth git-credential", nil).
		Expect(bashPrefix, "", nil)
	setRunner(t, fake)

	replaceStdin(t, "")
	getOut := captureStdout(t)
	if err := SessionStart(); err != nil {
		_ = getOut()
		t.Fatalf("SessionStart: %v", err)
	}
	_ = getOut()

	calls := fake.Calls()
	if len(calls) != 3 {
		t.Errorf("runner calls = %d, want 3 (gh auth setup-git + credential check + proxy probe)", len(calls))
		return
	}
	if !strings.Contains(calls[0].Argv[2], "gh auth setup-git") {
		t.Errorf("first call = %q, want gh auth setup-git", calls[0].Argv[2])
	}
	if calls[1].Argv[0] != "git" || !strings.Contains(strings.Join(calls[1].Argv, " "), "credential.https://github.com.helper") {
		t.Errorf("second call = %v, want git config --get-all credential check", calls[1].Argv)
	}
}
