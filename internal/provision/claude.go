package provision

import (
	"context"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
	"github.com/AlexShchuka/mirabilis/internal/tgtoken"
)

const FileClaudeToken = tgtoken.FilenameClaude

func ClaudeTokenPath() string {
	return filepath.Join(claudeDir(), FileClaudeToken)
}

func WriteClaudeToken(token string) error {
	_ = runtime.KeychainStore("claude-token", token)
	return writeClaudeTokenFile(token)
}

func writeClaudeTokenFile(token string) error {
	cd := claudeDir()
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return err
	}
	dest := ClaudeTokenPath()
	tmp, err := os.CreateTemp(cd, ".claude-token-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(token + "\n"); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dest)
}

func ReadClaudeTokenFile() string {
	return tgtoken.ReadFile(tgtoken.FilenameClaude)
}

func ClaudeCredentialsConflict(ctx context.Context, r runner.Runner) bool {
	out, err := r.Container(ctx, "bash", "-lc", `test -f "$HOME/.claude/.credentials.json" && echo yes || echo no`)
	if err != nil {
		return false
	}
	return len(out) >= 3 && out[:3] == "yes"
}
