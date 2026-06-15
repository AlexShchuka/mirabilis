// Package bus defines shared message and event types for TUI component communication.
package bus

import (
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/obs"
)

type NodeID string

func (n NodeID) Child(name string) NodeID {
	if n == "" {
		return NodeID(name)
	}
	return NodeID(string(n) + "/" + name)
}

func (n NodeID) Contains(target NodeID) bool {
	if n == "" || target == n {
		return true
	}
	return strings.HasPrefix(string(target), string(n)+"/")
}

type Envelope struct {
	Msg any
	To  NodeID
}

type MenuChosen struct {
	Action string
}

type StepEventKind int

const (
	StepStarted StepEventKind = iota
	StepLine
	StepDone
	StepFailed
	StepSkipped
	StepWaiting
)

type StepEvent struct {
	Argv []string
	Step string
	Line string
	Kind StepEventKind
}

type ScreenPush struct {
	Model any
}

type ScreenPop struct{}

type ScreenResult struct {
	Value  any
	Values map[string][]string
}

type StatusChanged struct {
	Snapshot obs.Snapshot
}

type CopyRequest struct {
	Text string
}

type ChromeTick struct {
	Frame int
}
