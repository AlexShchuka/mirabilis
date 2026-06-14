package localllm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type stubCompleter struct {
	text string
	err  error
}

func (s *stubCompleter) Complete(_ context.Context, _ string, _ Opts) (string, error) {
	return s.text, s.err
}

func connectInMemory(t *testing.T, c Completer) *mcp.ClientSession {
	t.Helper()
	ctx := t.Context()
	srv := NewServer(c)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	ct, st := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	sess, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func TestOffloadToolSuccess(t *testing.T) {
	sess := connectInMemory(t, &stubCompleter{text: "the answer"})
	res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "offload",
		Arguments: map[string]any{"prompt": "what is 1+1?"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("result IsError=true: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("empty content")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	if text.Text != "the answer" {
		t.Errorf("text = %q, want %q", text.Text, "the answer")
	}
}

func TestOffloadToolDegradedOnError(t *testing.T) {
	errMsg := "local model unavailable at http://host.docker.internal:1234/v1: connection refused"
	sess := connectInMemory(t, &stubCompleter{err: errors.New(errMsg)})
	res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "offload",
		Arguments: map[string]any{"prompt": "ping"},
	})
	if err != nil {
		t.Fatalf("CallTool protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for unavailable endpoint")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	if !strings.Contains(text.Text, "local model unavailable") {
		t.Errorf("error text = %q, want degraded sentinel", text.Text)
	}
}

func TestOffloadToolSanitizesLiveArtifact(t *testing.T) {
	rawFromModel := "PONG<turn|><turn|><turn|>"
	sess := connectInMemory(t, &stubCompleter{text: rawFromModel})
	res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "offload",
		Arguments: map[string]any{"prompt": "reply one word: PONG"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("result IsError=true: %+v", res.Content)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	const want = "PONG"
	if text.Text != want {
		t.Errorf("offload tool returned %q, want %q (control tokens not stripped)", text.Text, want)
	}
}

func TestOffloadToolEmptyPrompt(t *testing.T) {
	sess := connectInMemory(t, &stubCompleter{text: "should not be reached"})
	res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "offload",
		Arguments: map[string]any{"prompt": ""},
	})
	if err != nil {
		t.Fatalf("CallTool protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for empty prompt")
	}
}
