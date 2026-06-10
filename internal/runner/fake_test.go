package runner

import (
	"context"
	"errors"
	"testing"
)

func TestFakeRunner_NilFuncsReturnSilentSuccess(t *testing.T) {
	f := &FakeRunner{}
	out, err := f.Host(context.Background(), "cmd", "arg1")
	if err != nil {
		t.Errorf("Host with nil HostFunc: err = %v, want nil", err)
	}
	if out != "" {
		t.Errorf("Host with nil HostFunc: out = %q, want empty", out)
	}
	out, err = f.Container(context.Background(), "arg1", "arg2")
	if err != nil {
		t.Errorf("Container with nil ContFunc: err = %v, want nil", err)
	}
	if out != "" {
		t.Errorf("Container with nil ContFunc: out = %q, want empty", out)
	}
}

func TestFakeRunner_FuncsReceiveArgs(t *testing.T) {
	var gotName string
	var gotArgs []string
	f := &FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			gotName = name
			gotArgs = args
			return "host-out", nil
		},
	}
	out, err := f.Host(context.Background(), "mycmd", "a", "b")
	if err != nil {
		t.Fatalf("Host: %v", err)
	}
	if out != "host-out" {
		t.Errorf("Host out = %q, want host-out", out)
	}
	if gotName != "mycmd" {
		t.Errorf("HostFunc name = %q, want mycmd", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "a" || gotArgs[1] != "b" {
		t.Errorf("HostFunc args = %v, want [a b]", gotArgs)
	}

	var gotContArgs []string
	errBoom := errors.New("boom")
	f.ContFunc = func(args []string) (string, error) {
		gotContArgs = args
		return "", errBoom
	}
	_, err = f.Container(context.Background(), "x", "y")
	if !errors.Is(err, errBoom) {
		t.Errorf("Container err = %v, want boom", err)
	}
	if len(gotContArgs) != 2 || gotContArgs[0] != "x" || gotContArgs[1] != "y" {
		t.Errorf("ContFunc args = %v, want [x y]", gotContArgs)
	}
}

func TestFakeRunner_RepoReturnsRepoVal(t *testing.T) {
	f := &FakeRunner{RepoVal: "/my/repo"}
	if got := f.Repo(); got != "/my/repo" {
		t.Errorf("Repo() = %q, want /my/repo", got)
	}
	f2 := &FakeRunner{}
	if got := f2.Repo(); got != "" {
		t.Errorf("Repo() with zero value = %q, want empty", got)
	}
}
