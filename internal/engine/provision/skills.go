package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const ccSkillsGolang = "samber/cc-skills-golang"

type skillsStep struct {
	d Deps
}

func (s *skillsStep) Meta() pipeline.Meta { return installMeta("skills", "Claude skills") }

func (s *skillsStep) skillsDir() string { return filepath.Join(s.d.claudeDir(), "skills") }

func (s *skillsStep) Check(ctx context.Context) (bool, error) {
	catalog := readLines(s.d.Cfg.SkillsTxt())
	if len(catalog) == 0 {
		return true, nil
	}
	selected := s.d.selectedSkills()
	if len(selected) == 0 {
		return true, nil
	}
	for _, entry := range catalog {
		if !selected[entry] {
			continue
		}
		if entry == ccSkillsGolang {
			if !s.golangSkillsPresent() {
				return false, nil
			}
			continue
		}
		parts := strings.SplitN(entry, "/", 2)
		if len(parts) != 2 {
			continue
		}
		if !exists(filepath.Join(s.skillsDir(), parts[1])) {
			return false, nil
		}
	}
	return true, nil
}

func (s *skillsStep) golangSkillsPresent() bool {
	entries, err := os.ReadDir(s.skillsDir())
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "golang-") && e.IsDir() {
			return true
		}
	}
	return false
}

func (s *skillsStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	catalog := readLines(s.d.Cfg.SkillsTxt())
	if len(catalog) == 0 {
		return nil
	}
	selected := s.d.selectedSkills()
	if len(selected) == 0 {
		return nil
	}
	if err := os.MkdirAll(s.skillsDir(), 0o755); err != nil {
		return fmt.Errorf("mkdir skills: %w", err)
	}
	var errs []error
	for _, entry := range catalog {
		if !selected[entry] {
			continue
		}
		if entry == ccSkillsGolang {
			if err := s.installGolangSkills(ctx, out); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", entry, err))
			}
			continue
		}
		if err := s.installGitSkill(ctx, out, entry); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (s *skillsStep) installGolangSkills(ctx context.Context, out chan<- pipeline.Event) error {
	if s.golangSkillsPresent() {
		return nil
	}
	return s.d.stream(ctx, "skills", out,
		"gh", "skill", "install", ccSkillsGolang, "--all", "-f", "--dir", s.skillsDir(),
	)
}

func (s *skillsStep) installGitSkill(ctx context.Context, out chan<- pipeline.Event, entry string) error {
	if !s.d.argvOK(ctx, "git", "version") {
		return nil
	}
	parts := strings.SplitN(entry, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	owner, repo := parts[0], parts[1]
	dir := filepath.Join(s.skillsDir(), repo)
	if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
		return s.d.stream(ctx, "skills", out, "git", "-C", dir, "pull", "--ff-only")
	}
	if !exists(dir) {
		url := "https://github.com/" + owner + "/" + repo + ".git"
		return s.d.stream(ctx, "skills", out, "git", "clone", "--depth", "1", url, dir)
	}
	return nil
}
