package provision

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

const (
	FileHarness         = ".mirabilis-harness"
	FilePluginsDisabled = ".mirabilis-plugins-disabled"
	FileTheme           = ".mirabilis-theme"

	HarnessSkip    = "skip"
	HarnessInstall = "install"
)

func ReadHarnessChoiceContainer(ctx context.Context, r runner.Runner) string {
	out, _ := r.Container(ctx, "bash", "-lc",
		fmt.Sprintf(`cat "$HOME/.claude/%s" 2>/dev/null`, FileHarness))
	return strings.TrimSpace(out)
}

func WriteHarnessChoiceContainer(ctx context.Context, r runner.Runner, value string) error {
	_, err := r.Container(ctx, "bash", "-lc",
		fmt.Sprintf(`printf '%%s\n' %s > "$HOME/.claude/%s"`, value, FileHarness))
	return err
}

func ReadDisabledPluginsContainer(ctx context.Context, r runner.Runner) string {
	out, _ := r.Container(ctx, "bash", "-lc",
		fmt.Sprintf(`cat "$HOME/.claude/%s" 2>/dev/null`, FilePluginsDisabled))
	return out
}

func WriteDisabledPluginsContainer(ctx context.Context, r runner.Runner, content string) error {
	_, err := r.Container(ctx, "env", "MDIS="+content,
		"bash", "-lc",
		fmt.Sprintf(`printf '%%s' "$MDIS" > "$HOME/.claude/%s"`, FilePluginsDisabled))
	return err
}

func themeFilePath() string {
	return filepath.Join(claudeDir(), FileTheme)
}
