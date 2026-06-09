package provision

import (
	"context"
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
		warn("mkdir skills", err)
		return nil
	}

	icDir := filepath.Join(skillsDir, "interview-coach")
	if fi, err := os.Stat(filepath.Join(icDir, ".git")); err == nil && fi.IsDir() {
		if _, err := r.Host(ctx, "git", "-C", icDir, "pull", "--ff-only"); err != nil {
			warn("interview-coach pull", err)
		}
		return nil
	}

	if _, err := os.Stat(icDir); os.IsNotExist(err) {
		if _, err := r.Host(ctx, "git", "clone", "--depth", "1",
			"https://github.com/noamseg/interview-coach-skill.git", icDir); err != nil {
			warn("interview-coach skill not installed", err)
		}
	}
	return nil
}
