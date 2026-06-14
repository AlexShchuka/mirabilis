// Package exec provides subprocess execution primitives: streaming, PTY, and fake runners.
package exec

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type Spec struct {
	Stdin io.Reader
	Dir   string
	Argv  []string
	Env   []string
}

type EventKind int

const (
	KindStarted EventKind = iota
	KindStdout
	KindStderr
	KindExited
)

type Event struct {
	Err  error
	Line string
	Argv []string
	Kind EventKind
	Code int
}

type Runner interface {
	Stream(ctx context.Context, spec Spec) <-chan Event
}

func Run(ctx context.Context, r Runner, spec Spec) (string, error) {
	var stdout, stderr []string
	var exitErr error
	for ev := range r.Stream(ctx, spec) {
		switch ev.Kind {
		case KindStdout:
			stdout = append(stdout, ev.Line)
		case KindStderr:
			stderr = append(stderr, ev.Line)
		case KindExited:
			exitErr = ev.Err
		case KindStarted:
		}
	}
	out := strings.Join(stdout, "\n")
	if exitErr == nil {
		return out, nil
	}
	if len(stderr) > 0 {
		return out, fmt.Errorf("%w: %s", exitErr, strings.Join(stderr, "\n"))
	}
	return out, exitErr
}
