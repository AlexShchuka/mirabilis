package provision

import (
	"context"
	"fmt"
	"os"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type mcpEntry struct {
	name      string
	transport string
	url       string
	args      []string
}

func EnsureMCP(ctx context.Context, r runner.Runner) error {
	if _, err := r.Container(ctx, "bash", "-lc", "command -v claude"); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: claude not on PATH; skipping MCP registration\n")
		return nil
	}

	entries := []mcpEntry{
		{name: "context7", transport: "http", url: "https://mcp.context7.com/mcp"},
		{name: "sequential-thinking", transport: "stdio", args: []string{"npx", "-y", "@modelcontextprotocol/server-sequential-thinking"}},
	}

	if _, err := r.Container(ctx, "bash", "-lc", "command -v uvx"); err == nil {
		entries = append(entries,
			mcpEntry{name: "arxiv-mcp-server", transport: "stdio", args: []string{"uvx", "arxiv-mcp-server"}},
			mcpEntry{name: "docling", transport: "stdio", args: []string{"uvx", "--from", "docling-mcp[local]", "docling-mcp-server", "--transport", "stdio"}},
		)
	} else {
		fmt.Fprintf(os.Stderr, "[provision] WARN: uvx not on PATH; skipping arxiv-mcp-server and docling MCP servers\n")
	}

	for _, e := range entries {
		_, _ = r.Container(ctx, "claude", "mcp", "remove", e.name, "--scope", "user")

		var addArgs []string
		addArgs = append(addArgs, "mcp", "add", "--scope", "user", "--transport", e.transport)
		switch e.transport {
		case "http":
			addArgs = append(addArgs, e.name, e.url)
		case "stdio":
			addArgs = append(addArgs, e.name, "--")
			addArgs = append(addArgs, e.args...)
		}

		if _, err := r.Container(ctx, append([]string{"claude"}, addArgs...)...); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: failed to register %s: %v\n", e.name, err)
		} else {
			fmt.Fprintf(os.Stderr, "[provision] registered %s (%s)\n", e.name, e.transport)
		}
	}

	_, _ = r.Container(ctx, "claude", "mcp", "list")
	return nil
}
