// Package plugins installs Claude plugins from a resolved plan, idempotently.
package plugins

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision/reconcile"
)

const NeuroMatrix = "neuro-matrix@neuro-matrix"

type Plan struct {
	Marketplaces []string
	Units        []string
	Enabled      map[string]any
	Configured   bool
}

func Base(p string) string {
	if i := strings.Index(p, "@"); i >= 0 {
		return p[:i]
	}
	return p
}

func BuildPlan(catalog []string, disabled map[string]bool, harnessSkip bool, marketplaces []string) Plan {
	enabled := make(map[string]any)
	if !harnessSkip {
		enabled[NeuroMatrix] = true
	}
	var units []string
	for _, p := range catalog {
		if disabled[p] {
			continue
		}
		enabled[p] = true
		units = append(units, p)
	}
	return Plan{Marketplaces: marketplaces, Units: units, Enabled: enabled, Configured: len(catalog) > 0}
}

type SettingsIO struct {
	Read   func() (map[string]any, error)
	Write  func(map[string]any) error
	Exists func() bool
}

type Installer struct {
	ScriptOK func(ctx context.Context, script string) bool
	Output   func(ctx context.Context, argv ...string) (string, error)
	Stream   func(ctx context.Context, step string, out chan<- pipeline.Event, argv ...string) error
	Script   func(ctx context.Context, step string, out chan<- pipeline.Event, script string) error
	Settings SettingsIO
}

func (i Installer) Satisfied(ctx context.Context, plan Plan) (bool, error) {
	if !i.ScriptOK(ctx, "command -v claude") || !plan.Configured {
		return true, nil
	}
	listed, _ := i.Output(ctx, "claude", "plugin", "list")
	for _, p := range plan.Units {
		if !strings.Contains(listed, Base(p)) {
			return false, nil
		}
	}
	if !i.Settings.Exists() {
		return true, nil
	}
	m, err := i.Settings.Read()
	if err != nil {
		return true, nil
	}
	enabled, _ := m["enabledPlugins"].(map[string]any)
	return reflect.DeepEqual(enabled, plan.Enabled), nil
}

func (i Installer) Apply(ctx context.Context, out chan<- pipeline.Event, plan Plan) error {
	if !i.ScriptOK(ctx, "command -v claude") || !plan.Configured {
		return nil
	}
	_ = i.Script(ctx, "plugins", out, `mkdir -p "$HOME/.cache/tmp"`)
	var errs []error
	for _, market := range plan.Marketplaces {
		if err := i.Stream(ctx, "plugins", out, "claude", "plugin", "marketplace", "add", market); err != nil {
			errs = append(errs, fmt.Errorf("marketplace add %s: %w", market, err))
		}
	}
	listed, _ := i.Output(ctx, "claude", "plugin", "list")
	have := make(map[string]bool, len(plan.Units))
	for _, p := range plan.Units {
		have[p] = strings.Contains(listed, Base(p))
	}
	errs = append(errs, reconcile.Missing(plan.Units, have, func(p string) error {
		script := fmt.Sprintf(`TMPDIR="$HOME/.cache/tmp" claude plugin install %q --scope user`, p)
		if err := i.Script(ctx, "plugins", out, script); err != nil {
			return fmt.Errorf("plugin install %s: %w", p, err)
		}
		return nil
	}))
	errs = append(errs, i.writeEnabled(plan.Enabled))
	return errors.Join(errs...)
}

func (i Installer) writeEnabled(enabled map[string]any) error {
	if !i.Settings.Exists() {
		return nil
	}
	m, err := i.Settings.Read()
	if err != nil {
		return nil
	}
	m["enabledPlugins"] = enabled
	return i.Settings.Write(m)
}
