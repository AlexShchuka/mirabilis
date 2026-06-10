package runner

import (
	"context"
	"fmt"
)

var _ Runner = (*FakeRunner)(nil)

type FakeRunner struct {
	HostFunc func(name string, args []string) (string, error)
	ContFunc func(args []string) (string, error)
	RepoVal  string
}

func (f *FakeRunner) Repo() string { return f.RepoVal }

func (f *FakeRunner) Host(_ context.Context, name string, args ...string) (string, error) {
	if f.HostFunc == nil {
		return "", fmt.Errorf("FakeRunner: unexpected Host call: %s %v", name, args)
	}
	return f.HostFunc(name, args)
}

func (f *FakeRunner) Container(_ context.Context, args ...string) (string, error) {
	if f.ContFunc == nil {
		return "", fmt.Errorf("FakeRunner: unexpected Container call: %v", args)
	}
	return f.ContFunc(args)
}
