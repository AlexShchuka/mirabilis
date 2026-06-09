package main

import (
	"strings"
	"testing"
)

func TestHandoffArgvUsesInteractiveDockerExec(t *testing.T) {
	argv := handoffArgv("/usr/local/bin/docker", "tok123", "/tmp/sp.md")

	if argv[1] != "exec" || argv[2] != "-it" {
		t.Errorf("handoff must use `docker exec -it` for a real PTY, got %v", argv[1:3])
	}
	if argv[3] != "mirabilis" {
		t.Errorf("target container = %q, want mirabilis", argv[3])
	}

	joined := strings.Join(argv, " ")
	want := "/usr/local/bin/docker exec -it mirabilis " +
		"env GITHUB_PERSONAL_ACCESS_TOKEN=tok123 COLORTERM=truecolor TERM=xterm-256color " +
		"claude --dangerously-skip-permissions --append-system-prompt-file /tmp/sp.md"
	if joined != want {
		t.Errorf("handoffArgv =\n  %q\nwant\n  %q", joined, want)
	}
}
