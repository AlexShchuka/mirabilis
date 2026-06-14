package provision

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const localOffloadMCPName = "local-offload"

type localLLMStep struct {
	d        Deps
	selfPath string
}

func (s *localLLMStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     "local-offload",
		Title:    "Local LLM offload MCP",
		Optional: true,
		Kind:     pipeline.Auto,
		Timeout:  carryTimeout,
	}
}

func (s *localLLMStep) Check(ctx context.Context) (bool, error) {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return true, nil
	}
	return s.registered(ctx), nil
}

func (s *localLLMStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return nil
	}
	if s.registered(ctx) {
		return nil
	}
	self, err := s.resolveSelf()
	if err != nil {
		return fmt.Errorf("local-offload mcp: resolve self: %w", err)
	}
	argv := []string{
		"claude", "mcp", "add",
		"--scope", "user",
		"--transport", "stdio",
		localOffloadMCPName, "--",
		self, "localllm", "serve",
	}
	return s.d.stream(ctx, "local-offload", out, argv...)
}

func (s *localLLMStep) resolveSelf() (string, error) {
	if s.selfPath != "" {
		return s.selfPath, nil
	}
	return os.Executable()
}

func (s *localLLMStep) registered(ctx context.Context) bool {
	out, err := s.d.output(ctx, "claude", "mcp", "get", localOffloadMCPName)
	return err == nil && strings.Contains(out, localOffloadMCPName)
}
