package provision

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func parseMCPList(out string) map[string]bool {
	registered := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := strings.SplitN(line, " ", 2)[0]
		if name != "" {
			registered[name] = true
		}
	}
	return registered
}

type mcpStep struct {
	d Deps
}

func (s *mcpStep) Meta() pipeline.Meta { return carryMeta("mcp", "MCP servers") }

func (s *mcpStep) entries(ctx context.Context) ([]config.MCPEntry, error) {
	all, err := config.ReadMCPCatalog(s.d.Repo)
	if err != nil {
		return nil, err
	}
	hasUvx := s.d.scriptOK(ctx, "command -v uvx")
	var entries []config.MCPEntry
	for _, e := range all {
		if len(e.Args) > 0 && e.Args[0] == "uvx" && !hasUvx {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *mcpStep) Check(ctx context.Context) (bool, error) {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return true, nil
	}
	entries, err := s.entries(ctx)
	if err != nil {
		return false, err
	}
	listOut, _ := s.d.output(ctx, "claude", "mcp", "list")
	registered := parseMCPList(listOut)
	for _, e := range entries {
		if !registered[e.Name] {
			return false, nil
		}
	}
	return true, nil
}

func (s *mcpStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return nil
	}
	entries, err := s.entries(ctx)
	if err != nil {
		return err
	}
	listOut, _ := s.d.output(ctx, "claude", "mcp", "list")
	registered := parseMCPList(listOut)
	var errs []error
	for _, e := range entries {
		if registered[e.Name] {
			continue
		}
		argv := []string{"claude", "mcp", "add", "--scope", "user", "--transport", e.Transport}
		switch e.Transport {
		case "http":
			argv = append(argv, e.Name, e.URL)
		case "stdio":
			argv = append(argv, e.Name, "--")
			argv = append(argv, e.Args...)
		}
		if err := s.d.stream(ctx, "mcp", out, argv...); err != nil {
			errs = append(errs, fmt.Errorf("register %s: %w", e.Name, err))
		}
	}
	return errors.Join(errs...)
}
