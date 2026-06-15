// Package pipeline sequences and executes provisioning steps with streaming event output.
package pipeline

import (
	"context"
	"errors"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

type Kind int

const (
	Auto Kind = iota
	Interactive
	Terminal
	// Handoff is a terminal hand-off whose Check is NOT a skip-gate; the pipeline always runs it.
	Handoff
)

type RetryPolicy struct {
	Attempts int
	Delay    time.Duration
}

type Meta struct {
	Name     string
	Title    string
	Deps     []string
	Retry    RetryPolicy
	Timeout  time.Duration
	Kind     Kind
	Optional bool
}

type Result struct {
	Value     any
	Cancelled bool
}

type Command interface {
	Meta() Meta
	Check(ctx context.Context) (bool, error)
	Run(ctx context.Context, out chan<- Event, in <-chan Result) error
}

type EventKind int

const (
	EvStepStarted EventKind = iota
	EvSpawn
	EvLine
	EvDone
	EvFailed
	EvSkipped
	EvWaiting
	EvPipelineDone
)

type Event struct {
	Payload any
	Err     error
	Step    string
	Line    string
	Argv    []string
	Env     []string
	Kind    EventKind
	Failed  bool
}

var ErrCancelled = errors.New("step cancelled")

const LineSatisfied = "satisfied"

func Forward(step string, out chan<- Event, ev exec.Event) {
	switch ev.Kind {
	case exec.KindStarted:
		out <- Event{Kind: EvSpawn, Step: step, Argv: ev.Argv}
	case exec.KindStdout, exec.KindStderr:
		out <- Event{Kind: EvLine, Step: step, Line: ev.Line}
	}
}
