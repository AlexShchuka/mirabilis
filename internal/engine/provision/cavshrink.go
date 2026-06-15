package provision

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type caveShrinkStep struct {
	d Deps
}

func (s *caveShrinkStep) Meta() pipeline.Meta {
	return carryMeta("cav-shrink", "MCP description shrink")
}

func (s *caveShrinkStep) shrinkTargets() ([]config.MCPEntry, error) {
	all, err := config.ReadMCPCatalog(s.d.Repo)
	if err != nil {
		return nil, err
	}
	var out []config.MCPEntry
	for _, e := range all {
		if e.Shrink && e.Transport == "stdio" {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *caveShrinkStep) Check(_ context.Context) (bool, error) {
	targets, err := s.shrinkTargets()
	if err != nil || len(targets) == 0 {
		return true, err
	}
	m, readErr := readJSON(s.d.claudeJSONPath())
	if readErr != nil {
		return false, nil
	}
	applied, _ := m["mcpShrinkApplied"].(map[string]any)
	for _, e := range targets {
		if v, _ := applied[e.Name].(bool); !v {
			return false, nil
		}
	}
	return true, nil
}

func (s *caveShrinkStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	targets, err := s.shrinkTargets()
	if err != nil || len(targets) == 0 {
		return err
	}
	path := s.d.claudeJSONPath()
	m := map[string]any{}
	if existing, readErr := readJSON(path); readErr == nil {
		m = existing
	}
	applied, _ := m["mcpShrinkApplied"].(map[string]any)
	if applied == nil {
		applied = map[string]any{}
	}
	var errs []error
	for _, e := range targets {
		if v, _ := applied[e.Name].(bool); v {
			continue
		}
		upstream := strings.Join(e.Args, " ")
		if runErr := s.d.stream(ctx, "cav-shrink", out, "npx", "caveman-shrink", "--with-mcp-shrink="+upstream); runErr != nil {
			errs = append(errs, fmt.Errorf("shrink %s: %w", e.Name, runErr))
			continue
		}
		applied[e.Name] = true
	}
	m["mcpShrinkApplied"] = applied
	if writeErr := writeJSON(path, m); writeErr != nil {
		errs = append(errs, fmt.Errorf("write sentinel: %w", writeErr))
	}
	return errors.Join(errs...)
}
