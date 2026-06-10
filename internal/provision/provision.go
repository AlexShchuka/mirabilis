package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func warn(step string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[provision] WARN: %s: %v\n", step, err)
}

func warnCount(step string, err error, warned *int) {
	if err != nil {
		*warned++
	}
	warn(step, err)
}

func writeProvisionStatus(warned, total int) {
	cd := claudeDir()
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return
	}
	var content string
	if warned == 0 {
		content = "ok"
	} else {
		content = fmt.Sprintf("%d/%d warned", warned, total)
	}
	_ = os.WriteFile(filepath.Join(cd, ".mirabilis-provision-status"), []byte(content), 0o644)
}

func ensureAll(ctx context.Context, r runner.Runner, cfg config.Config) {
	warned := 0
	total := 0

	step := func(name string, err error) {
		total++
		warnCount(name, err, &warned)
	}

	step("settings", EnsureSettings(cfg))
	step("theme", EnsureTheme(cfg))
	step("memory", EnsureMemory())
	step("git identity", EnsureGitIdentity(ctx, r))
	step("plugins", EnsurePlugins(ctx, r, cfg))
	step("claude-hud config", EnsureHudConfig(cfg))
	ok, err := HarnessInstalled(ctx, r)
	step("harness check", err)
	if !ok {
		step("harness", EnsureHarness(ctx, r))
	}
	step("mcp", EnsureMCP(ctx, r))
	step("skills", EnsureSkills(ctx, r))
	step("rtk", EnsureRTK(ctx, r, cfg))
	step("rtk config", EnsureRTKConfig(cfg))
	step("headroom", EnsureHeadroom(ctx, r))

	if warned > 0 {
		fmt.Fprintf(os.Stderr, "[provision] %d of %d steps warned\n", warned, total)
	}
	writeProvisionStatus(warned, total)
}

func Create(ctx context.Context, r runner.Runner, cfg config.Config) error {
	ensureAll(ctx, r, cfg)
	return nil
}

func Start(ctx context.Context, r runner.Runner, cfg config.Config) error {
	ensureAll(ctx, r, cfg)
	warn("harness symlink re-assert", relinkHarness(ctx, r))
	return nil
}
