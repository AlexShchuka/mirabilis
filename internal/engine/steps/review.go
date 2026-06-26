package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	reviewHarvestName         = "review-harvest"
	reviewPresentName         = "review-present"
	ecosystemDirRel           = "ecosystem"
	changesetManifestFileName = "changeset-manifest.json"
	reviewTimeout             = 2 * time.Minute
)

type changesetManifest struct {
	SessionID string                  `json:"sessionId"`
	Timestamp string                  `json:"timestamp"`
	Repos     []changesetManifestRepo `json:"repos"`
	Session   changesetManifestSess   `json:"session"`
}

type changesetManifestRepo struct {
	Name  string   `json:"name"`
	Dir   string   `json:"dir"`
	Diff  string   `json:"diff"`
	Files []string `json:"files"`
}

type changesetManifestSess struct {
	TranscriptMinusTools string `json:"transcriptMinusTools"`
}

func Review(d Deps) []pipeline.Command {
	return []pipeline.Command{
		&reviewHarvestStep{d: d},
		&reviewPresentStep{d: d},
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func ecosystemRoot() string { return filepath.Join(homeDir(), ecosystemDirRel) }

func manifestPath() string {
	return filepath.Join(homeDir(), ".claude", changesetManifestFileName)
}

type reviewHarvestStep struct {
	d Deps
}

func (s *reviewHarvestStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    reviewHarvestName,
		Title:   "Harvest",
		Kind:    pipeline.Auto,
		Timeout: reviewTimeout,
	}
}

func (s *reviewHarvestStep) Check(ctx context.Context) (bool, error) {
	data, err := os.ReadFile(manifestPath())
	if err != nil {
		return false, nil
	}
	var m changesetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return false, nil
	}
	current, err := s.collect(ctx)
	if err != nil {
		return false, nil
	}
	return manifestReposEqual(m.Repos, current), nil
}

func (s *reviewHarvestStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	repos, err := s.collect(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	m := changesetManifest{
		SessionID: "review-" + now,
		Timestamp: now,
		Repos:     repos,
	}
	path, err := writeChangesetManifest(m)
	if err != nil {
		return fmt.Errorf("review: write manifest: %w", err)
	}
	out <- pipeline.Event{Kind: pipeline.EvLine, Step: reviewHarvestName, Line: fmt.Sprintf("manifest: %s (%d repo(s))", path, len(repos))}
	return nil
}

func (s *reviewHarvestStep) collect(ctx context.Context) ([]changesetManifestRepo, error) {
	root := ecosystemRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("review: read ecosystem root: %w", err)
	}
	var repos []changesetManifestRepo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if !reviewIsGitRepo(ctx, s.d, dir) {
			continue
		}
		diff := reviewRepoDiff(ctx, s.d, dir)
		files := reviewRepoFiles(ctx, s.d, dir)
		if diff == "" && len(files) == 0 {
			continue
		}
		repos = append(repos, changesetManifestRepo{
			Name:  e.Name(),
			Dir:   dir,
			Diff:  diff,
			Files: files,
		})
	}
	return repos, nil
}

func reviewIsGitRepo(ctx context.Context, d Deps, dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return false
	}
	_, err := exec.Run(ctx, d.Runner, exec.Spec{Argv: []string{"git", "-C", dir, "rev-parse", "--git-dir"}})
	return err == nil
}

func reviewRepoDiff(ctx context.Context, d Deps, dir string) string {
	out, err := exec.Run(ctx, d.Runner, exec.Spec{Argv: []string{"git", "-C", dir, "diff", "HEAD"}})
	if err != nil {
		d.Obs.Logger(reviewHarvestName).Warn("git diff failed", "dir", dir, "err", err)
		return ""
	}
	return out
}

func reviewRepoFiles(ctx context.Context, d Deps, dir string) []string {
	out, err := exec.Run(ctx, d.Runner, exec.Spec{Argv: []string{"git", "-C", dir, "status", "--porcelain"}})
	if err != nil || out == "" {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) <= 3 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}

func manifestReposEqual(a, b []changesetManifestRepo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Dir != b[i].Dir || a[i].Diff != b[i].Diff {
			return false
		}
		if !setsEqual(a[i].Files, b[i].Files) {
			return false
		}
	}
	return true
}

func writeChangesetManifest(m changesetManifest) (string, error) {
	path := manifestPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

type reviewPresentStep struct {
	d Deps
}

func (s *reviewPresentStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:  reviewPresentName,
		Title: "Review",
		Deps:  []string{reviewHarvestName},
		Kind:  pipeline.Terminal,
	}
}

func (s *reviewPresentStep) Check(context.Context) (bool, error) {
	return false, nil
}

func (s *reviewPresentStep) Run(_ context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	data, err := os.ReadFile(manifestPath())
	if err != nil {
		return fmt.Errorf("review: read manifest: %w", err)
	}
	var m changesetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("review: decode manifest: %w", err)
	}
	if len(m.Repos) == 0 {
		out <- pipeline.Event{Kind: pipeline.EvLine, Step: reviewPresentName, Line: "no local changes across ecosystem repos"}
		return nil
	}
	for _, r := range m.Repos {
		out <- pipeline.Event{Kind: pipeline.EvLine, Step: reviewPresentName, Line: fmt.Sprintf("%s: %d changed file(s)", r.Name, len(r.Files))}
	}
	return nil
}
