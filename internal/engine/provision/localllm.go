package provision

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/localllm"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	localOffloadMCPName = "local-offload"
	localLLMPingTimeout = 2 * time.Second
)

type localLLMStep struct {
	d        Deps
	selfPath string
	client   *http.Client
}

func (s *localLLMStep) reachable(ctx context.Context) bool {
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: localLLMPingTimeout}
	}
	pingCtx, cancel := context.WithTimeout(ctx, localLLMPingTimeout)
	defer cancel()
	_, err := localllm.DiscoverModel(pingCtx, config.LocalLLMEffectiveBaseURL(), client)
	return err == nil
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
	if !s.reachable(ctx) {
		return true, nil
	}
	self, err := s.resolveSelf()
	if err != nil {
		return false, fmt.Errorf("local-offload mcp: resolve self: %w", err)
	}
	return s.registeredAt(ctx, self), nil
}

func (s *localLLMStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return nil
	}
	if !s.reachable(ctx) {
		if s.d.Log != nil {
			s.d.Log.Info("local LLM host unreachable, skipping offload MCP registration",
				"url", config.LocalLLMEffectiveBaseURL())
		}
		return nil
	}
	self, err := s.resolveSelf()
	if err != nil {
		return fmt.Errorf("local-offload mcp: resolve self: %w", err)
	}
	if s.registeredAt(ctx, self) {
		return nil
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

func (s *localLLMStep) registeredAt(ctx context.Context, self string) bool {
	out, err := s.d.output(ctx, "claude", "mcp", "get", localOffloadMCPName)
	return err == nil && strings.Contains(out, self)
}
