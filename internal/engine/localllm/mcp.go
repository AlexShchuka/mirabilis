package localllm

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type offloadArgs struct {
	Prompt    string `json:"prompt"                   jsonschema:"the prompt to complete"`
	System    string `json:"system,omitempty"         jsonschema:"optional system message"`
	MaxTokens int    `json:"max_tokens,omitempty"     jsonschema:"optional max token cap"`
}

func NewServer(c Completer) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "local-offload", Version: "1"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "offload",
		Description: "Send a prompt to the host LM Studio model and return the completion text.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args offloadArgs) (*mcp.CallToolResult, any, error) {
		if args.Prompt == "" {
			return nil, nil, fmt.Errorf("prompt must not be empty")
		}
		text, err := c.Complete(ctx, args.Prompt, Opts{System: args.System, MaxTokens: args.MaxTokens})
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
	return s
}

func ServeStdio(ctx context.Context, c Completer) error {
	s := NewServer(c)
	return s.Run(ctx, &mcp.StdioTransport{})
}
