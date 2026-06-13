package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func containerArgv(args ...string) []string {
	return append([]string{"docker", "exec", sandbox.ContainerName}, args...)
}

const streamTailLines = 10

func stream(step string, out chan<- pipeline.Event, events <-chan exec.Event) error {
	var exitErr error
	var tail []string
	for ev := range events {
		switch ev.Kind {
		case exec.KindExited:
			exitErr = ev.Err
		case exec.KindStdout, exec.KindStderr:
			if line := strings.TrimSpace(ev.Line); line != "" {
				tail = append(tail, line)
				if len(tail) > streamTailLines {
					tail = tail[len(tail)-streamTailLines:]
				}
			}
		}
		pipeline.Forward(step, out, ev)
	}
	if exitErr != nil && len(tail) > 0 {
		return fmt.Errorf("%w: %s", exitErr, strings.Join(tail, "; "))
	}
	return exitErr
}

func awaitResume(ctx context.Context, in <-chan pipeline.Result) (pipeline.Result, error) {
	select {
	case r := <-in:
		if r.Cancelled {
			return r, pipeline.ErrCancelled
		}
		return r, nil
	case <-ctx.Done():
		return pipeline.Result{}, ctx.Err()
	}
}

func asStrings(step string, v any) ([]string, error) {
	s, ok := v.([]string)
	if !ok {
		return nil, fmt.Errorf("steps: %s: expected []string result, got %T", step, v)
	}
	return s, nil
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func setsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ca := slices.Clone(a)
	cb := slices.Clone(b)
	slices.Sort(ca)
	slices.Sort(cb)
	return slices.Equal(ca, cb)
}

func subtract(all, except []string) []string {
	var out []string
	for _, v := range all {
		if !slices.Contains(except, v) {
			out = append(out, v)
		}
	}
	return out
}

func dotenvRead(repo, key string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(repo, ".env"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, key+"="); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

func dotenvWrite(repo, key, value string) error {
	path := filepath.Join(repo, ".env")
	var keep []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, key+"=") {
				keep = append(keep, line)
			}
		}
	}
	out := strings.TrimRight(strings.Join(keep, "\n"), "\n")
	if out != "" {
		out += "\n"
	}
	out += key + "=" + value + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}
