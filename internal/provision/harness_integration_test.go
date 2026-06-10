//go:build integration

package provision

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestRelinkHarness_RepointsSymlinkInRealContainer(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx := t.Context()
	ctr, err := testcontainers.Run(ctx, "debian:bookworm-slim",
		testcontainers.WithCmd("sleep", "infinity"),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start container: %v", err)
	}

	execInCtr := func(args ...string) (string, error) {
		code, r, err := ctr.Exec(ctx, args, tcexec.Multiplexed())
		if err != nil {
			return "", err
		}
		out, _ := io.ReadAll(r)
		if code != 0 {
			return "", fmt.Errorf("exit %d: %s", code, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}

	seedCache := func(version string) {
		dir := "$HOME/.claude/plugins/cache/local/neuro-matrix/" + version
		if _, err := execInCtr("bash", "-lc", "mkdir -p "+dir); err != nil {
			t.Fatalf("seed cache %s: %v", version, err)
		}
	}

	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return execInCtr(args...)
		},
	}

	seedCache("1.0.0")
	if err := relinkHarness(ctx, r); err != nil {
		t.Fatalf("relinkHarness (first run): %v", err)
	}

	target1, err := execInCtr("bash", "-lc", `readlink "$HOME/.neuro-matrix"`)
	if err != nil {
		t.Fatalf("readlink after first run: %v", err)
	}
	if !strings.HasSuffix(target1, "/neuro-matrix/1.0.0") {
		t.Errorf("symlink target after first run = %q, want suffix /neuro-matrix/1.0.0", target1)
	}

	exportLine, err := execInCtr("bash", "-lc", `grep -c '^export CLAUDE_PLUGIN_ROOT="$HOME/.neuro-matrix"$' "$HOME/.bashrc"`)
	if err != nil {
		t.Fatalf("grep export after first run: %v", err)
	}
	if exportLine != "1" {
		t.Errorf("export line count after first run = %q, want 1", exportLine)
	}

	seedCache("2.0.0")
	if err := relinkHarness(ctx, r); err != nil {
		t.Fatalf("relinkHarness (second run): %v", err)
	}

	target2, err := execInCtr("bash", "-lc", `readlink "$HOME/.neuro-matrix"`)
	if err != nil {
		t.Fatalf("readlink after second run: %v", err)
	}
	if !strings.HasSuffix(target2, "/neuro-matrix/2.0.0") {
		t.Errorf("symlink target after repoint = %q, want suffix /neuro-matrix/2.0.0", target2)
	}

	exportLineAfter, err := execInCtr("bash", "-lc", `grep -c '^export CLAUDE_PLUGIN_ROOT="$HOME/.neuro-matrix"$' "$HOME/.bashrc"`)
	if err != nil {
		t.Fatalf("grep export after second run: %v", err)
	}
	if exportLineAfter != "1" {
		t.Errorf("export line count after second run = %q, want 1 (idempotent)", exportLineAfter)
	}
}
