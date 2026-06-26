package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const (
	changesetManifestFileName = "changeset-manifest.json"
	transcriptBlockText       = "text"
	transcriptRoleUser        = "user"
	transcriptRoleAssistant   = "assistant"
)

type Manifest struct {
	SessionID string          `json:"sessionId"`
	Timestamp string          `json:"timestamp"`
	Repos     []ManifestRepo  `json:"repos"`
	Session   ManifestSession `json:"session"`
}

type ManifestRepo struct {
	Name  string   `json:"name"`
	Dir   string   `json:"dir"`
	Diff  string   `json:"diff"`
	Files []string `json:"files"`
}

type ManifestSession struct {
	TranscriptMinusTools string `json:"transcriptMinusTools"`
}

type stopHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Timestamp      string `json:"timestamp"`
}

func changesetManifestPath() string {
	return filepath.Join(home(), ".claude", changesetManifestFileName)
}

func parseStopInput(data []byte) stopHookInput {
	var in stopHookInput
	if len(data) == 0 {
		return in
	}
	_ = json.Unmarshal(data, &in)
	return in
}

func harvest(ctx context.Context, in stopHookInput, timestamp string) Manifest {
	if in.SessionID == "" {
		fmt.Fprintln(os.Stderr, "[hook] WARN: harvest: empty sessionId from Stop stdin; manifest will carry an empty sessionId")
	}
	m := Manifest{
		SessionID: in.SessionID,
		Timestamp: timestamp,
		Repos:     collectRepos(ctx),
	}
	m.Session.TranscriptMinusTools = transcriptMinusTools(in.TranscriptPath)
	return m
}

func collectRepos(ctx context.Context) []ManifestRepo {
	root := ecosystemRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[hook] WARN: harvest: read ecosystem root %s: %v\n", root, err)
		}
		return nil
	}
	var repos []ManifestRepo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if !isGitRepo(ctx, dir) {
			continue
		}
		diff := repoDiff(ctx, dir)
		files := repoChangedFiles(ctx, dir)
		if diff == "" && len(files) == 0 {
			continue
		}
		repos = append(repos, ManifestRepo{
			Name:  e.Name(),
			Dir:   dir,
			Diff:  diff,
			Files: files,
		})
	}
	return repos
}

func repoDiff(ctx context.Context, dir string) string {
	out, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "-C", dir, "diff", "HEAD"}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: harvest: git diff in %s: %v\n", dir, err)
		return ""
	}
	return out
}

func repoChangedFiles(ctx context.Context, dir string) []string {
	out, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "-C", dir, "status", "--porcelain"}})
	if err != nil || out == "" {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) <= 3 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}

func transcriptMinusTools(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: harvest: read transcript %s: %v\n", path, err)
		return ""
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if text, ok := keptTranscriptText(line); ok {
			kept = append(kept, text)
		}
	}
	return strings.Join(kept, "\n")
}

func keptTranscriptText(line string) (string, bool) {
	var entry struct {
		Type    string          `json:"type"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return "", false
	}
	if entry.Type != transcriptRoleUser && entry.Type != transcriptRoleAssistant {
		return "", false
	}
	var msg struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return "", false
	}
	role := msg.Role
	if role == "" {
		role = entry.Type
	}

	var asString string
	if err := json.Unmarshal(msg.Content, &asString); err == nil {
		text := strings.TrimSpace(asString)
		if text == "" {
			return "", false
		}
		return role + ": " + text, true
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return "", false
	}
	var texts []string
	for _, b := range blocks {
		if b.Type != transcriptBlockText {
			continue
		}
		if t := strings.TrimSpace(b.Text); t != "" {
			texts = append(texts, t)
		}
	}
	if len(texts) == 0 {
		return "", false
	}
	return role + ": " + strings.Join(texts, "\n"), true
}

func writeManifest(m Manifest) (string, error) {
	path := changesetManifestPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
