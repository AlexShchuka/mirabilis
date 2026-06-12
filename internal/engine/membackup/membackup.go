package membackup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const (
	containerSrc = "mirabilis:/home/node/.claude/memory"
	saveSubdir   = ".mirabilis/saved-memory"
)

func Save(ctx context.Context, r exec.Runner, repo string) error {
	dst := filepath.Join(repo, saveSubdir)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("save memory: mkdir: %w", err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("save memory: clear: %w", err)
	}
	if _, err := exec.Run(ctx, r, exec.Spec{Argv: []string{"docker", "cp", containerSrc, dst}}); err != nil {
		return fmt.Errorf("save memory: docker cp: %w", err)
	}
	return nil
}
