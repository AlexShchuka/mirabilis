package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const (
	Node = "version"

	defaultBaseURL = "https://api.github.com"
	owner          = "AlexShchuka"
	repo           = "mirabilis"

	checkTimeout    = 3 * time.Second
	backoffInitial  = 30 * time.Second
	backoffCap      = 5 * time.Minute
	unknownVersion  = "unknown"
	acceptHeaderKey = "Accept"
	acceptHeaderVal = "application/vnd.github+json"
)

type Checker struct {
	client  *http.Client
	baseURL string
}

func NewChecker() *Checker {
	return &Checker{
		client:  &http.Client{Timeout: checkTimeout},
		baseURL: defaultBaseURL,
	}
}

func (c *Checker) Check(ctx context.Context, currentVersion string) (latestTag string, behind bool, err error) {
	apiURL := c.baseURL + "/repos/" + owner + "/" + repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("release: build request: %w", err)
	}
	req.Header.Set(acceptHeaderKey, acceptHeaderVal)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("release: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("release: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", false, fmt.Errorf("release: read body: %w", err)
	}

	var result struct {
		TagName   string `json:"tag_name"`
		TargetSHA string `json:"target_commitish"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false, fmt.Errorf("release: decode response: %w", err)
	}
	if result.TagName == "" {
		return "", false, fmt.Errorf("release: empty tag in response")
	}

	return result.TagName, isBehind(currentVersion, result.TargetSHA), nil
}

func isBehind(current, releaseSHA string) bool {
	current = strings.TrimSpace(current)
	releaseSHA = strings.TrimSpace(releaseSHA)
	if current == "" || releaseSHA == "" {
		return false
	}
	return !strings.HasPrefix(releaseSHA, current)
}

func (c *Checker) Run(ctx context.Context, o *obs.Obs, currentVersion string) {
	if currentVersion == "" || currentVersion == unknownVersion {
		return
	}
	backoff := backoffInitial
	for {
		tag, behind, err := c.Check(ctx, currentVersion)
		if err == nil && behind {
			o.Set(Node, obs.StateDegraded, tag)
			return
		}
		if err == nil {
			return
		}
		if !sleep(ctx, backoff) {
			return
		}
		backoff *= 2
		if backoff > backoffCap {
			backoff = backoffCap
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
