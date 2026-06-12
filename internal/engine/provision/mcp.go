package provision

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type mcpEntry struct {
	name      string
	transport string
	url       string
	args      []string
}

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

func (s *mcpStep) entries(ctx context.Context) []mcpEntry {
	entries := []mcpEntry{
		{name: "context7", transport: "http", url: "https://mcp.context7.com/mcp"},
		{name: "sequential-thinking", transport: "stdio", args: []string{"npx", "-y", "@modelcontextprotocol/server-sequential-thinking"}},
	}
	if s.d.scriptOK(ctx, "command -v uvx") {
		entries = append(entries,
			mcpEntry{name: "arxiv-mcp-server", transport: "stdio", args: []string{"uvx", "arxiv-mcp-server"}},
			mcpEntry{name: "docling", transport: "stdio", args: []string{"uvx", "--from", "docling-mcp[local]", "docling-mcp-server", "--transport", "stdio"}},
		)
	}
	return entries
}

func (s *mcpStep) Check(ctx context.Context) (bool, error) {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return true, nil
	}
	entries := s.entries(ctx)
	listOut, _ := s.d.output(ctx, "claude", "mcp", "list")
	registered := parseMCPList(listOut)
	for _, e := range entries {
		if !registered[e.name] {
			return false, nil
		}
	}
	return true, nil
}

func (s *mcpStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return nil
	}
	entries := s.entries(ctx)
	listOut, _ := s.d.output(ctx, "claude", "mcp", "list")
	registered := parseMCPList(listOut)
	var errs []error
	for _, e := range entries {
		if registered[e.name] {
			continue
		}
		argv := []string{"claude", "mcp", "add", "--scope", "user", "--transport", e.transport}
		switch e.transport {
		case "http":
			argv = append(argv, e.name, e.url)
		case "stdio":
			argv = append(argv, e.name, "--")
			argv = append(argv, e.args...)
		}
		if err := s.d.stream(ctx, "mcp", out, argv...); err != nil {
			errs = append(errs, fmt.Errorf("register %s: %w", e.name, err))
		}
	}
	return errors.Join(errs...)
}
