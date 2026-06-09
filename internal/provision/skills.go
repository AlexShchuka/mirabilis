package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func EnsureSkills(ctx context.Context, r runner.Runner) error {
	if _, err := r.Host(ctx, "git", "version"); err != nil {
		return nil
	}

	skillsDir := filepath.Join(claudeDir(), "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: mkdir skills: %v\n", err)
		return nil
	}

	icDir := filepath.Join(skillsDir, "interview-coach")
	if fi, err := os.Stat(filepath.Join(icDir, ".git")); err == nil && fi.IsDir() {
		if _, err := r.Host(ctx, "git", "-C", icDir, "pull", "--ff-only"); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: interview-coach pull: %v\n", err)
		}
		return nil
	}

	if _, err := os.Stat(icDir); os.IsNotExist(err) {
		if _, err := r.Host(ctx, "git", "clone", "--depth", "1",
			"https://github.com/noamseg/interview-coach-skill.git", icDir); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: interview-coach skill not installed — check network: %v\n", err)
		}
	}
	return nil
}
