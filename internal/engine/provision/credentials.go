package provision

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const credentialsFileName = ".credentials.json"

type credentialsStep struct {
	d Deps
}

func (s *credentialsStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    "claude-credentials",
		Title:   "Claude credentials guard",
		Kind:    pipeline.Auto,
		Timeout: 10 * time.Second,
	}
}

func (s *credentialsStep) path() string {
	return filepath.Join(s.d.claudeDir(), credentialsFileName)
}

func (s *credentialsStep) Check(_ context.Context) (bool, error) {
	_, err := os.Stat(s.path())
	return errors.Is(err, fs.ErrNotExist), nil
}

func (s *credentialsStep) Run(_ context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	path := s.path()
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}
	pipeline.Forward("claude-credentials", out, exec.Event{
		Kind: exec.KindStderr,
		Line: "removed ~/.claude/" + credentialsFileName + " — it would override the host auth chain (I1)",
	})
	return nil
}
