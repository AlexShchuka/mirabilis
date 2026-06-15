package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const claudeWorkspaceDir = "/workspace"

func (d Deps) claudeJSONPath() string {
	return filepath.Join(filepath.Dir(d.claudeDir()), ".claude.json")
}

type onboardingStep struct {
	d       Deps
	version string
}

func (s *onboardingStep) Meta() pipeline.Meta { return carryMeta("onboarding", "Claude onboarding") }

func (s *onboardingStep) claudeVersion() string {
	if s.version != "" {
		return s.version
	}
	v, err := s.d.output(context.Background(), "claude", "--version")
	if err != nil || v == "" {
		return ""
	}
	for i, c := range v {
		if c == ' ' {
			return v[:i]
		}
	}
	return v
}

func (s *onboardingStep) Check(_ context.Context) (bool, error) {
	m, err := readJSON(s.d.claudeJSONPath())
	if err != nil {
		return false, nil
	}
	completed, _ := m["hasCompletedOnboarding"].(bool)
	skip, _ := m["skipDangerousModePermissionPrompt"].(bool)
	projects, _ := m["projects"].(map[string]any)
	if !completed || !skip || projects == nil {
		return false, nil
	}
	proj, _ := projects[claudeWorkspaceDir].(map[string]any)
	trusted, _ := proj["hasTrustDialogAccepted"].(bool)
	projOnboard, _ := proj["hasCompletedProjectOnboarding"].(bool)
	return trusted && projOnboard, nil
}

func (s *onboardingStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	path := s.d.claudeJSONPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for .claude.json: %w", err)
	}
	m := map[string]any{}
	if existing, err := readJSON(path); err == nil {
		m = existing
	}
	m["hasCompletedOnboarding"] = true
	m["skipDangerousModePermissionPrompt"] = true
	if ver := s.claudeVersion(); ver != "" {
		m["lastOnboardingVersion"] = ver
	}
	projects, _ := m["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	proj, _ := projects[claudeWorkspaceDir].(map[string]any)
	if proj == nil {
		proj = map[string]any{}
	}
	proj["hasTrustDialogAccepted"] = true
	proj["hasCompletedProjectOnboarding"] = true
	projects[claudeWorkspaceDir] = proj
	m["projects"] = projects
	return writeJSON(path, m)
}
