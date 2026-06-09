package runner

import "context"

type FakeRunner struct {
	RepoVal  string
	HostFunc func(name string, args []string) (string, error)
	ContFunc func(args []string) (string, error)
}

func (f *FakeRunner) Repo() string { return f.RepoVal }

func (f *FakeRunner) Host(_ context.Context, name string, args ...string) (string, error) {
	if f.HostFunc == nil {
		return "", nil
	}
	return f.HostFunc(name, args)
}

func (f *FakeRunner) Container(_ context.Context, args ...string) (string, error) {
	if f.ContFunc == nil {
		return "", nil
	}
	return f.ContFunc(args)
}
