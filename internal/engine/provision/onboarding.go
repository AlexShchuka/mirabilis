package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const claudeWorkspaceDir = "/workspace"

func (d Deps) claudeJSONPath() string {
	return filepath.Join(filepath.Dir(d.claudeDir()), ".claude.json")
}

var onboardingBeltKeys = map[string]any{
	"hasCompletedOnboarding":        true,
	"hasCompletedProjectOnboarding": true,
	"lastOnboardingVersion":         json.Number("1"),
	"projectOnboardingSeenCount":    json.Number("1"),
	"bypassPermissionsModeAccepted": true,
}

type onboardingStep struct {
	d Deps
}

func (s *onboardingStep) Meta() pipeline.Meta { return carryMeta("onboarding", "Claude onboarding") }

func (s *onboardingStep) Check(_ context.Context) (bool, error) {
	m, err := readJSON(s.d.claudeJSONPath())
	if err != nil {
		return false, nil
	}
	for k, want := range onboardingBeltKeys {
		if m[k] != want {
			return false, nil
		}
	}
	projects, _ := m["projects"].(map[string]any)
	if projects == nil {
		return false, nil
	}
	proj, _ := projects[claudeWorkspaceDir].(map[string]any)
	trusted, _ := proj["hasTrustDialogAccepted"].(bool)
	if !trusted {
		return false, nil
	}
	sdpp, _ := m["skipDangerousModePermissionPrompt"].(bool)
	if !sdpp {
		return false, nil
	}
	sm, err := readJSON(s.d.settingsPath())
	if err != nil {
		return false, nil
	}
	smSDPP, _ := sm["skipDangerousModePermissionPrompt"].(bool)
	return smSDPP, nil
}

func (s *onboardingStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	path := s.d.claudeJSONPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for .claude.json: %w", err)
	}
	err := updateJSON(path, func(m map[string]any) error {
		for k, v := range onboardingBeltKeys {
			m[k] = v
		}
		m["skipDangerousModePermissionPrompt"] = true
		projects, _ := m["projects"].(map[string]any)
		if projects == nil {
			projects = map[string]any{}
		}
		proj, _ := projects[claudeWorkspaceDir].(map[string]any)
		if proj == nil {
			proj = map[string]any{}
		}
		proj["hasTrustDialogAccepted"] = true
		projects[claudeWorkspaceDir] = proj
		m["projects"] = projects
		return nil
	})
	if err != nil {
		return fmt.Errorf("write .claude.json: %w", err)
	}
	settingsPath := s.d.settingsPath()
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for settings.json: %w", err)
	}
	return updateJSON(settingsPath, func(sm map[string]any) error {
		sm["skipDangerousModePermissionPrompt"] = true
		return nil
	})
}
