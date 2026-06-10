package runtime

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func DoVSCode(ctx context.Context, r runner.Runner) error {
	code, err := resolveCode()
	if err != nil {
		return err
	}
	if !ContainerRunning(ctx, r) {
		up := exec.CommandContext(ctx, "devcontainer", "up", "--workspace-folder", r.Repo())
		up.Env = ComposeEnv(r.Repo())
		if e := up.Run(); e != nil {
			return e
		}
	}
	enc := hex.EncodeToString([]byte(`{"containerName":"/mirabilis"}`))
	uri := "vscode-remote://attached-container+" + enc + "/workspace"
	return exec.Command(code, "--folder-uri", uri).Run()
}

func resolveCode() (string, error) {
	if p, err := exec.LookPath("code"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, b := range []string{
		"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
		filepath.Join(home, "Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"),
		"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
		"/usr/share/code/bin/code",
		"/snap/bin/code",
		"/var/lib/flatpak/exports/bin/com.visualstudio.code",
		filepath.Join(home, ".local/share/flatpak/exports/bin/com.visualstudio.code"),
	} {
		if fi, err := os.Stat(b); err == nil && !fi.IsDir() {
			return b, nil
		}
	}
	return "", fmt.Errorf("VS Code not found — install it from https://code.visualstudio.com")
}
