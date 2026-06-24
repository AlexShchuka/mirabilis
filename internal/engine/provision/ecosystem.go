package provision

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const ecosystemDirRel = "ecosystem"

var ecosystemRepos = []string{
	"darwin",
	"mirabilis",
	"SolitaryEquilibriumShield",
	"neuro-matrix",
}

type ecosystemStep struct {
	d Deps
}

func (s *ecosystemStep) Meta() pipeline.Meta { return installMeta("ecosystem", "Ecosystem repos") }

func (s *ecosystemStep) root() string { return filepath.Join(s.d.Home, ecosystemDirRel) }

func (s *ecosystemStep) repoDir(name string) string { return filepath.Join(s.root(), name) }

func (s *ecosystemStep) cloned(ctx context.Context, name string) bool {
	return s.d.scriptOK(ctx, fmt.Sprintf(`test -d %q`, filepath.Join(s.repoDir(name), ".git")))
}

func (s *ecosystemStep) Check(ctx context.Context) (bool, error) {
	if !s.d.argvOK(ctx, "gh", "auth", "status") {
		return true, nil
	}
	for _, name := range ecosystemRepos {
		if !s.cloned(ctx, name) {
			return false, nil
		}
	}
	return true, nil
}

func (s *ecosystemStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.argvOK(ctx, "gh", "auth", "status") {
		return nil
	}
	if err := s.d.streamScript(ctx, "ecosystem", out, "gh auth setup-git"); err != nil {
		return fmt.Errorf("gh auth setup-git: %w", err)
	}
	if err := s.d.streamScript(ctx, "ecosystem", out, fmt.Sprintf(`mkdir -p %q`, s.root())); err != nil {
		return fmt.Errorf("mkdir ecosystem: %w", err)
	}
	for _, name := range ecosystemRepos {
		if err := s.sync(ctx, out, name); err != nil {
			return fmt.Errorf("ecosystem %s: %w", name, err)
		}
	}
	return nil
}

func (s *ecosystemStep) sync(ctx context.Context, out chan<- pipeline.Event, name string) error {
	dir := s.repoDir(name)
	if s.cloned(ctx, name) {
		return s.d.streamScript(ctx, "ecosystem", out, fmt.Sprintf(`git -C %q pull --ff-only`, dir))
	}
	url := fmt.Sprintf("https://github.com/AlexShchuka/%s.git", name)
	return s.d.streamScript(ctx, "ecosystem", out, fmt.Sprintf(`git clone %q %q`, url, dir))
}
