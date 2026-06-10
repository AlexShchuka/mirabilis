package provision

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func readSkillCatalog(cfg config.Config) []string {
	f, err := os.Open(cfg.SkillsTxt())
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func readSelectedSkills() []string {
	raw := readSkillsFile()
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func EnsureSkills(ctx context.Context, r runner.Runner, cfg config.Config) error {
	catalog := readSkillCatalog(cfg)
	if len(catalog) == 0 {
		return nil
	}

	selected := readSelectedSkills()
	if len(selected) == 0 {
		return nil
	}

	if _, err := r.Host(ctx, "git", "version"); err != nil {
		return nil
	}

	selectedSet := make(map[string]bool, len(selected))
	for _, s := range selected {
		selectedSet[s] = true
	}

	skillsDir := filepath.Join(claudeDir(), "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		warn("mkdir skills", err)
		return nil
	}

	for _, entry := range catalog {
		if !selectedSet[entry] {
			continue
		}
		parts := strings.SplitN(entry, "/", 2)
		if len(parts) != 2 {
			continue
		}
		owner, repo := parts[0], parts[1]
		dir := filepath.Join(skillsDir, repo)
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
			if _, err := r.Host(ctx, "git", "-C", dir, "pull", "--ff-only"); err != nil {
				warn(entry+" pull", err)
			}
			continue
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			url := "https://github.com/" + owner + "/" + repo + ".git"
			if _, err := r.Host(ctx, "git", "clone", "--depth", "1", url, dir); err != nil {
				warn(entry+" clone", err)
			}
		}
	}
	return nil
}
