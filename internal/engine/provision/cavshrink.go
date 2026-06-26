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
	alreadyApplied := map[string]bool{}
	if existing, readErr := readJSON(path); readErr == nil {
		if cur, _ := existing["mcpShrinkApplied"].(map[string]any); cur != nil {
			for name, v := range cur {
				if b, _ := v.(bool); b {
					alreadyApplied[name] = true
				}
			}
		}
	}
	var errs []error
	freshlyApplied := map[string]bool{}
	for _, e := range targets {
		if alreadyApplied[e.Name] {
			continue
		}
		upstream := strings.Join(e.Args, " ")
		if runErr := s.d.stream(ctx, "cav-shrink", out, "npx", "caveman-shrink", "--with-mcp-shrink="+upstream); runErr != nil {
			errs = append(errs, fmt.Errorf("shrink %s: %w", e.Name, runErr))
			continue
		}
		freshlyApplied[e.Name] = true
	}
	if writeErr := updateJSON(path, func(m map[string]any) error {
		cur, _ := m["mcpShrinkApplied"].(map[string]any)
		if cur == nil {
			cur = map[string]any{}
		}
		for name := range freshlyApplied {
			cur[name] = true
		}
		m["mcpShrinkApplied"] = cur
		return nil
	}); writeErr != nil {
		errs = append(errs, fmt.Errorf("write sentinel: %w", writeErr))
	}
	return errors.Join(errs...)
}
