package sandbox

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func (s *Sandbox) OpenVSCode(ctx context.Context) error {
	code, err := resolveCode()
	if err != nil {
		return err
	}
	if c, ierr := s.docker.Inspect(ctx); ierr != nil || !c.Running {
		if uerr := drain(s.Up(ctx)); uerr != nil {
			return uerr
		}
	}
	_, err = exec.Run(ctx, s.runner, exec.Spec{Argv: vscodeArgv(code), Dir: s.repo})
	return err
}

func vscodeArgv(code string) []string {
	enc := hex.EncodeToString([]byte(`{"containerName":"/` + ContainerName + `"}`))
	return []string{code, "--folder-uri", "vscode-remote://attached-container+" + enc + "/"}
}

func resolveCode() (string, error) {
	if p, err := lookPath("code"); err == nil {
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
	return "", errors.New("VS Code not found — install it from https://code.visualstudio.com")
}

func lookPath(name string) (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", errors.New("sandbox: " + name + " not found in PATH")
}
