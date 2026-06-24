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
	_, _ = io.ReadAll(os.Stdin)

	ctx := context.Background()
	committed := commitEcosystem(ctx)
	if committed == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "[ecosystem] no local changes to commit")
		return nil
	}
	_, _ = fmt.Fprintf(os.Stdout, "[ecosystem] committed changes in %d repo(s) locally — review and push when ready\n", committed)
	return nil
}

func commitEcosystem(ctx context.Context) int {
	root := ecosystemRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	msg := "ecosystem auto-snapshot " + stamp
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if !isGitRepo(ctx, dir) || !hasChanges(ctx, dir) {
			continue
		}
		if commitRepo(ctx, dir, msg) {
			count++
		}
	}
	return count
}

func isGitRepo(ctx context.Context, dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	_, err = exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "-C", dir, "rev-parse", "--git-dir"}})
	return err == nil
}

func hasChanges(ctx context.Context, dir string) bool {
	out, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "-C", dir, "status", "--porcelain"}})
	return err == nil && out != ""
}

func commitRepo(ctx context.Context, dir, msg string) bool {
	if _, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "-C", dir, "add", "-A"}}); err != nil {
		fmt.Fprintf(os.Stderr, "[ecosystem] WARN: git add in %s: %v\n", dir, err)
		return false
	}
	if _, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "-C", dir, "commit", "-m", msg}}); err != nil {
		fmt.Fprintf(os.Stderr, "[ecosystem] WARN: git commit in %s: %v\n", dir, err)
		return false
	}
	return true
}
