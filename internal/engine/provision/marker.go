package provision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func StartMarkerHash(fingerprint, sessionKey string) string {
	sum := sha256.Sum256([]byte(fingerprint + sessionKey))
	return hex.EncodeToString(sum[:])
}

type createMarkerStep struct {
	d Deps
}

func (s *createMarkerStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    "create-marker",
		Title:   "Provision marker",
		Kind:    pipeline.Auto,
		Timeout: 10 * time.Second,
	}
}

func (s *createMarkerStep) Check(_ context.Context) (bool, error) {
	data, err := os.ReadFile(filepath.Join(s.d.claudeDir(), CreateMarkerName))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(data)) == createMarkerOK, nil
}

func (s *createMarkerStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if err := os.MkdirAll(s.d.claudeDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.d.claudeDir(), CreateMarkerName), []byte(createMarkerOK+"\n"), 0o644)
}

type startMarkerStep struct {
	d Deps
}

func (s *startMarkerStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    "start-marker",
		Title:   "Session marker",
		Deps:    []string{"settings-env"},
		Kind:    pipeline.Auto,
		Timeout: 10 * time.Second,
	}
}

func (s *startMarkerStep) hash() string {
	return StartMarkerHash(os.Getenv("MIRABILIS_VERSION"), s.d.SessionKey)
}

func (s *startMarkerStep) Check(_ context.Context) (bool, error) {
	data, err := os.ReadFile(filepath.Join(s.d.claudeDir(), StartMarkerName))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(data)) == s.hash(), nil
}

func (s *startMarkerStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if err := os.MkdirAll(s.d.claudeDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.d.claudeDir(), StartMarkerName), []byte(s.hash()+"\n"), 0o644)
}
