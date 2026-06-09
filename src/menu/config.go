package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func pluginCatalog(ctx context.Context, r Runner) []string {
	raw, _ := r.Container(ctx, "bash", "-lc", `sed -e '/^#/d' -e '/^[[:space:]]*$/d' /opt/mirabilis/config/plugins.txt 2>/dev/null`)
	return splitLines(raw)
}

func pluginsDisabled(ctx context.Context, r Runner) []string {
	raw, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-plugins-disabled" 2>/dev/null`)
	return splitLines(raw)
}

func writePluginsDisabled(ctx context.Context, r Runner, disabled []string) error {
	_, err := r.Container(ctx, "env", "MDIS="+strings.Join(disabled, "\n"),
		"bash", "-lc", `printf '%s' "$MDIS" > "$HOME/.claude/.mirabilis-plugins-disabled"`)
	return err
}

func applyHarness(ctx context.Context, r Runner, choice string) error {
	switch choice {
	case "off":
		_, err := r.Container(ctx, "bash", "-lc", `echo skip > "$HOME/.claude/.mirabilis-harness"`)
		return err
	case "on":
		_, err := r.Container(ctx, "bash", "-lc", `echo install > "$HOME/.claude/.mirabilis-harness"`)
		return err
	case "reinstall":
		if _, err := r.Container(ctx, "bash", "-lc", `echo install > "$HOME/.claude/.mirabilis-harness"`); err != nil {
			return err
		}
		_, err := r.Container(ctx, "bash", "/usr/local/bin/harness-reinstall.sh")
		return err
	}
	return nil
}

func doVSCodeCmd(ctx context.Context, r Runner) tea.Cmd {
	return func() tea.Msg {
		if err := doVSCode(ctx, r); err != nil {
			return backToMenuMsg{notice: "VS Code: " + err.Error()}
		}
		return backToMenuMsg{}
	}
}

func doVSCode(ctx context.Context, r Runner) error {
	code, err := resolveCode()
	if err != nil {
		return err
	}
	if !containerRunning(ctx, r) {
		up := exec.CommandContext(ctx, "devcontainer", "up", "--workspace-folder", r.Repo())
		up.Env = composeEnv(r.Repo())
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
	} {
		if fi, err := os.Stat(b); err == nil && !fi.IsDir() {
			return b, nil
		}
	}
	return "", fmt.Errorf("VS Code not found — install it from https://code.visualstudio.com")
}
