// Package skills installs Claude skills from a resolved list of units, idempotently.
package skills

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision/reconcile"
)

type Unit struct {
	Repo  string
	Skill string
}

func Units(groups []config.SkillGroup, selected map[string]bool) []Unit {
	if len(selected) == 0 {
		return nil
	}
	var units []Unit
	for _, g := range groups {
		if !selected[g.Name] {
			continue
		}
		for _, sk := range g.Skills {
			units = append(units, Unit{Repo: g.Repo, Skill: sk})
		}
	}
	return units
}

type Installer struct {
	OK     func(ctx context.Context, argv ...string) bool
	Output func(ctx context.Context, argv ...string) (string, error)
	Stream func(ctx context.Context, step string, out chan<- pipeline.Event, argv ...string) error
}

func (i Installer) Satisfied(ctx context.Context, units []Unit) (bool, error) {
	if len(units) == 0 || !i.OK(ctx, "gh", "--version") {
		return true, nil
	}
	have := i.installed(ctx)
	for _, u := range units {
		if !have[u] {
			return false, nil
		}
	}
	return true, nil
}

func (i Installer) Apply(ctx context.Context, out chan<- pipeline.Event, units []Unit) error {
	if len(units) == 0 || !i.OK(ctx, "gh", "--version") {
		return nil
	}
	return reconcile.Missing(units, i.installed(ctx), func(u Unit) error {
		return i.Stream(ctx, "skills", out,
			"gh", "skill", "install", u.Repo, u.Skill,
			"--agent", "claude-code", "--scope", "user", "--force")
	})
}

func (i Installer) installed(ctx context.Context) map[Unit]bool {
	raw, err := i.Output(ctx, "gh", "skill", "list", "--agent", "claude-code", "--scope", "user", "--json", "skillName,sourceURL")
	if err != nil {
		return nil
	}
	var items []struct {
		SkillName string `json:"skillName"`
		SourceURL string `json:"sourceURL"`
	}
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	have := make(map[Unit]bool, len(items))
	for _, it := range items {
		have[Unit{Repo: repoSlug(it.SourceURL), Skill: it.SkillName}] = true
	}
	return have
}

func repoSlug(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "http://github.com/")
	return url
}
