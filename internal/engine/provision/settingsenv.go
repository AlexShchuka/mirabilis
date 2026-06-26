package provision

import (
	"context"
	"os"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type settingsEnvStep struct {
	d Deps
}

func (s *settingsEnvStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:    "settings-env",
		Title:   "Auth chain settings",
		Deps:    []string{"headroom"},
		Kind:    pipeline.Auto,
		Timeout: 30 * time.Second,
	}
}

func (s *settingsEnvStep) Check(_ context.Context) (bool, error) {
	m, err := readJSON(s.d.settingsPath())
	if err != nil {
		return false, nil
	}
	env, _ := m[settingsEnvKey].(map[string]any)
	if env == nil {
		return false, nil
	}
	if env[settingsURLKey] != settingsBaseURL {
		return false, nil
	}
	if s.d.SessionKey != "" && env[settingsTokenKey] != s.d.SessionKey {
		return false, nil
	}
	return true, nil
}

func (s *settingsEnvStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	path := s.d.settingsPath()
	if err := os.MkdirAll(s.d.claudeDir(), 0o755); err != nil {
		return err
	}
	return updateJSON(path, func(m map[string]any) error {
		env, _ := m[settingsEnvKey].(map[string]any)
		if env == nil {
			env = map[string]any{}
		}
		env[settingsURLKey] = settingsBaseURL
		if s.d.SessionKey != "" {
			env[settingsTokenKey] = s.d.SessionKey
		}
		m[settingsEnvKey] = env
		return nil
	})
}
