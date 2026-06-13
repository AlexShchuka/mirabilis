package provision

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	fileHarness         = ".mirabilis-harness"
	filePluginsDisabled = ".mirabilis-plugins-disabled"
	fileTheme           = ".mirabilis-theme"
	fileSkills          = ".mirabilis-skills"
	harnessSkip         = "skip"
	harnessInstall      = "install"
	carryTimeout        = 5 * time.Minute
	installTimeout      = 15 * time.Minute
)

func carryCreate(d Deps) []pipeline.Command {
	return []pipeline.Command{
		&settingsStep{d: d},
		&themeStep{d: d},
		&memoryStep{d: d},
		&gitIdentityStep{d: d},
		&hudStep{d: d},
		&mcpStep{d: d},
		&rtkStep{d: d},
		&rtkConfigStep{d: d},
	}
}

func carryStart(d Deps) []pipeline.Command {
	return []pipeline.Command{
		&harnessStep{d: d},
		&pluginsStep{d: d},
		&skillsStep{d: d},
	}
}

func carryPlugins(d Deps) []pipeline.Command { return []pipeline.Command{&pluginsStep{d: d}} }

func carrySkills(d Deps) []pipeline.Command { return []pipeline.Command{&skillsStep{d: d}} }

func carryMeta(name, title string) pipeline.Meta {
	return pipeline.Meta{
		Name:     name,
		Title:    title,
		Optional: true,
		Kind:     pipeline.Auto,
		Timeout:  carryTimeout,
	}
}

func installMeta(name, title string) pipeline.Meta {
	m := carryMeta(name, title)
	m.Timeout = installTimeout
	return m
}

type cmdRunner struct {
	r exec.Runner
}

func (d Deps) cmd() cmdRunner { return cmdRunner{r: d.Runner} }

func (c cmdRunner) output(ctx context.Context, argv ...string) (string, error) {
	return exec.Run(ctx, c.r, exec.Spec{Argv: argv})
}

func (c cmdRunner) argvOK(ctx context.Context, argv ...string) bool {
	_, err := c.output(ctx, argv...)
	return err == nil
}

func (c cmdRunner) scriptOK(ctx context.Context, script string) bool {
	return c.argvOK(ctx, "bash", "-lc", script)
}

func (c cmdRunner) stream(ctx context.Context, step string, out chan<- pipeline.Event, argv ...string) error {
	for ev := range c.r.Stream(ctx, exec.Spec{Argv: argv}) {
		pipeline.Forward(step, out, ev)
		if ev.Kind == exec.KindExited {
			return ev.Err
		}
	}
	return nil
}

func (c cmdRunner) streamScript(ctx context.Context, step string, out chan<- pipeline.Event, script string) error {
	return c.stream(ctx, step, out, "bash", "-lc", script)
}

func (d Deps) harnessChoice() string {
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), fileHarness))
	if err != nil {
		return harnessInstall
	}
	return strings.TrimSpace(string(data))
}

func (d Deps) themePath() string { return filepath.Join(d.claudeDir(), fileTheme) }

func (d Deps) disabledPlugins() map[string]bool {
	return readSet(filepath.Join(d.claudeDir(), filePluginsDisabled))
}

func (d Deps) selectedSkills() map[string]bool {
	return readSet(filepath.Join(d.claudeDir(), fileSkills))
}

func readSet(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out[line] = true
		}
	}
	return out
}

func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
