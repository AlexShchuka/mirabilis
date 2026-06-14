package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func PostToolUseFailure() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: PostToolUseFailure: read stdin: %v\n", err)
		return nil
	}

	var payload struct {
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
		ToolResponse struct {
			Stdout string `json:"stdout"`
		} `json:"tool_response"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}

	combined := payload.ToolInput.Command + " " + payload.ToolResponse.Stdout
	tokens := tokenize(combined)
	if len(tokens) == 0 {
		return nil
	}

	memDir := filepath.Join(home(), ".claude", "memory")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		return nil
	}

	var matched []string
	totalBytes := 0
outer:
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "MEMORY.md" {
			continue
		}
		fileData, err := os.ReadFile(filepath.Join(memDir, e.Name()))
		if err != nil {
			continue
		}
		for _, bullet := range readBullets(fileData) {
			if !matchesBullet(bullet, tokens) {
				continue
			}
			if len(matched) >= postToolUseFailureBulletCap {
				break outer
			}
			if totalBytes+len(bullet) > postToolUseFailureByteCap {
				break outer
			}
			matched = append(matched, bullet)
			totalBytes += len(bullet)
		}
	}

	if len(matched) == 0 {
		return nil
	}

	additionalContext := "Relevant memory:\n" + strings.Join(matched, "\n")
	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUseFailure",
			"additionalContext": additionalContext,
		},
	})
	_, _ = os.Stdout.Write(out)
	return nil
}
