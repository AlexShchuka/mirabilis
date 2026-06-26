package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func SessionEnd() error {
	data, _ := io.ReadAll(os.Stdin)
	in := parseStopInput(data)

	ctx := context.Background()
	timestamp := in.Timestamp
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	m := harvest(ctx, in, timestamp)

	path, err := writeManifest(m)
	if err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "[harvest] wrote changeset manifest for %d repo(s) to %s\n", len(m.Repos), path)
	return nil
}

func isGitRepo(ctx context.Context, dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	_, err = exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "-C", dir, "rev-parse", "--git-dir"}})
	return err == nil
}
