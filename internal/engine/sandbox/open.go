package sandbox

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

var errOpenDisabled = errors.New("sandbox: browser open disabled")

func OpenURL(ctx context.Context, runner exec.Runner, url string) error {
	argv, err := openArgv(url)
	if errors.Is(err, errOpenDisabled) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = exec.Run(ctx, runner, exec.Spec{Argv: argv})
	return err
}

func openArgv(url string) ([]string, error) {
	if os.Getenv("MIRABILIS_NO_BROWSER") != "" {
		return nil, errOpenDisabled
	}
	browser, browserSet := os.LookupEnv("BROWSER")
	if browserSet {
		if browser == "" {
			return nil, errOpenDisabled
		}
		return []string{browser, url}, nil
	}
	if isWSL() {
		wslview, err := lookPath("wslview")
		if err != nil {
			return nil, errors.New("sandbox: 'wslview' not found in PATH; set $BROWSER")
		}
		return []string{wslview, url}, nil
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
