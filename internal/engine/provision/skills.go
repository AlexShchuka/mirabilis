package provision

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

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
	if !s.d.argvOK(ctx, "git", "version") {
		return true, nil
	}
	for _, entry := range catalog {
		if !selected[entry] {
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

func (s *skillsStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	catalog := readLines(s.d.Cfg.SkillsTxt())
	if len(catalog) == 0 {
		return nil
	}
	selected := s.d.selectedSkills()
	if len(selected) == 0 {
		return nil
	}
	if !s.d.argvOK(ctx, "git", "version") {
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
		parts := strings.SplitN(entry, "/", 2)
		if len(parts) != 2 {
			continue
		}
		owner, repo := parts[0], parts[1]
		dir := filepath.Join(s.skillsDir(), repo)
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
			if err := s.d.stream(ctx, "skills", out, "git", "-C", dir, "pull", "--ff-only"); err != nil {
				errs = append(errs, fmt.Errorf("%s pull: %w", entry, err))
			}
			continue
		}
		if !exists(dir) {
			url := "https://github.com/" + owner + "/" + repo + ".git"
			if err := s.d.stream(ctx, "skills", out, "git", "clone", "--depth", "1", url, dir); err != nil {
				errs = append(errs, fmt.Errorf("%s clone: %w", entry, err))
			}
		}
	}
	return errors.Join(errs...)
}
