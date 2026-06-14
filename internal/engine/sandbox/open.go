package sandbox

import (
	"context"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func OpenURL(ctx context.Context, runner exec.Runner, url string) error {
	argv, err := openArgv(url)
	if err != nil {
		return err
	}
	_, err = exec.Run(ctx, runner, exec.Spec{Argv: argv})
	return err
}

func openArgv(url string) ([]string, error) {
	if browser := os.Getenv("BROWSER"); browser != "" {
		return []string{browser, url}, nil
	}
	if isWSL() {
		return []string{"wslview", url}, nil
	}
	return platformOpen(url)
}

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}
