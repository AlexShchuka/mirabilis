// Package localllm bridges in-container Claude Code to a host-local LM Studio instance.
package localllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Opts struct {
	System    string
	MaxTokens int
}

type Completer interface {
	Complete(ctx context.Context, prompt string, opts Opts) (string, error)
}

type HTTPAdapter struct {
	BaseURL   string
	Model     string
	Timeout   time.Duration
	MaxTokens int
	Client    *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
	Stream    bool          `json:"stream"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *HTTPAdapter) Complete(ctx context.Context, prompt string, opts Opts) (string, error) {
	msgs := []chatMessage{}
	if opts.System != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: opts.System})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: prompt})

	maxTok := a.MaxTokens
	if opts.MaxTokens > 0 {
		maxTok = opts.MaxTokens
	}

	body, err := json.Marshal(chatRequest{
		Model:     a.Model,
		Messages:  msgs,
		MaxTokens: maxTok,
		Stream:    false,
	})
	if err != nil {
		return "", fmt.Errorf("localllm: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, a.Timeout)
	defer cancel()

	url := strings.TrimRight(a.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("localllm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("local model unavailable at %s: %w", a.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("localllm: http %d from %s", resp.StatusCode, a.BaseURL)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("localllm: read response: %w", err)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("localllm: decode response: %w", err)
	}
	if cr.Error != nil && cr.Error.Message != "" {
		return "", fmt.Errorf("localllm: model error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("localllm: no choices in response from %s", a.BaseURL)
	}
	return cr.Choices[0].Message.Content, nil
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func DiscoverModel(ctx context.Context, baseURL string, client *http.Client) (string, error) {
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("localllm: build models request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("localllm: models request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("localllm: read models response: %w", err)
	}
	var mr modelsResponse
	if err := json.Unmarshal(raw, &mr); err != nil {
		return "", fmt.Errorf("localllm: decode models: %w", err)
	}
	if len(mr.Data) == 0 {
		return "", fmt.Errorf("localllm: no models available at %s", baseURL)
	}
	return mr.Data[0].ID, nil
}

var (
	templateTagRE = regexp.MustCompile(`<\|[^|>]*\|>`)
	repeatedNewlines = regexp.MustCompile(`\n{3,}`)
)

func SanitizeOutput(s string) string {
	s = templateTagRE.ReplaceAllString(s, "")
	s = repeatedNewlines.ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)
	return s
}
