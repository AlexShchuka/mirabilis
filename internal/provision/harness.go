package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func HarnessInstalled(ctx context.Context, r runner.Runner) (bool, error) {
	pref, _ := os.ReadFile(filepath.Join(claudeDir(), ".mirabilis-harness"))
	if strings.TrimSpace(string(pref)) == "skip" {
		return true, nil
	}
	_, err := r.Container(ctx, "bash", "-lc", "claude plugin list 2>/dev/null | grep -q neuro-matrix")
	return err == nil, nil
}

func EnsureHarness(ctx context.Context, r runner.Runner) error {
	if _, err := r.Container(ctx, "bash", "-lc", "command -v claude"); err != nil {
		return nil
	}

	if _, err := r.Container(ctx, "claude", "plugin", "marketplace", "add", "AlexShchuka/neuro-matrix"); err != nil {
		if _, err2 := r.Container(ctx, "claude", "plugin", "marketplace", "update", "neuro-matrix"); err2 != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: marketplace add/update neuro-matrix: %v\n", err)
		}
	}

	if _, err := r.Container(ctx, "claude", "plugin", "install", "neuro-matrix@neuro-matrix", "--scope", "user"); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: plugin install neuro-matrix: %v\n", err)
	}

	if _, err := r.Container(ctx, "claude", "plugin", "update", "neuro-matrix"); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: plugin update neuro-matrix: %v\n", err)
	}

	if _, err := r.Container(ctx, "bash", "-lc", "claude plugin list 2>/dev/null | grep -q neuro-matrix"); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: neuro-matrix not installed after reinstall — check git/network\n")
		return nil
	}

	if err := relinkHarness(ctx, r); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: neuro-matrix symlink: %v\n", err)
	}
	return nil
}

func relinkHarness(ctx context.Context, r runner.Runner) error {
	_, err := r.Container(ctx, "bash", "-lc",
		`NM_DIR="$(printf '%s\n' "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ | sort -V | tail -n1)"; [ -d "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"`)
	return err
}
