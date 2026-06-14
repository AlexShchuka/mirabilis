package provision

import (
	"errors"
	"testing"
)

const testSelfPath = "/usr/local/bin/mirabilis"

func testLocalLLMStep(d Deps) *localLLMStep {
	return &localLLMStep{d: d, selfPath: testSelfPath}
}

func localLLMAddArgv() []string {
	return []string{
		"claude", "mcp", "add",
		"--scope", "user",
		"--transport", "stdio",
		"local-offload", "--",
		testSelfPath, "localllm", "serve",
	}
}

func TestLocalLLMCheckTrueWhenCLAUDEAbsent(t *testing.T) {
	d, f := testDeps(t)
	f.Expect(script(`command -v claude`), "", errors.New("not found"))
	step := testLocalLLMStep(d)
	if !checkStep(t, step) {
		t.Error("check should be true when claude CLI is absent (optional step)")
	}
}

func registeredOutput() string {
	return "local-offload stdio " + testSelfPath + " localllm serve"
}

func TestLocalLLMCheckFalseWhenNotRegistered(t *testing.T) {
	d, f := testDeps(t)
	f.Expect(script(`command -v claude`), "", nil)
	f.Expect([]string{"claude", "mcp", "get", "local-offload"}, "", errors.New("not found"))
	step := testLocalLLMStep(d)
	if checkStep(t, step) {
		t.Error("check should be false when local-offload is not registered")
	}
}

func TestLocalLLMCheckFalseWhenRegisteredAtDifferentPath(t *testing.T) {
	d, f := testDeps(t)
	f.Expect(script(`command -v claude`), "", nil)
	f.Expect([]string{"claude", "mcp", "get", "local-offload"}, "local-offload stdio /old/bin/mirabilis localllm serve", nil)
	step := testLocalLLMStep(d)
	if checkStep(t, step) {
		t.Error("check should be false when registered path differs from self (stale binary)")
	}
}

func TestLocalLLMCheckTrueWhenRegisteredAtCurrentPath(t *testing.T) {
	d, f := testDeps(t)
	f.Expect(script(`command -v claude`), "", nil)
	f.Expect([]string{"claude", "mcp", "get", "local-offload"}, registeredOutput(), nil)
	step := testLocalLLMStep(d)
	if !checkStep(t, step) {
		t.Error("check should be true when local-offload is registered at current self path")
	}
}

func TestLocalLLMRunRegisters(t *testing.T) {
	d, f := testDeps(t)
	f.Expect(script(`command -v claude`), "", nil)
	f.Expect([]string{"claude", "mcp", "get", "local-offload"}, "", errors.New("not found"))
	f.Expect(localLLMAddArgv(), "", nil)
	step := testLocalLLMStep(d)
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if r := f.Remaining(); r != 0 {
		t.Errorf("unused stubs after run: %d", r)
	}
}

func TestLocalLLMRunIdempotentWhenAlreadyRegistered(t *testing.T) {
	d, f := testDeps(t)
	f.Expect(script(`command -v claude`), "", nil)
	f.Expect([]string{"claude", "mcp", "get", "local-offload"}, registeredOutput(), nil)
	step := testLocalLLMStep(d)
	if err := runStep(t, step); err != nil {
		t.Fatalf("run should be no-op when already registered: %v", err)
	}
	if r := f.Remaining(); r != 0 {
		t.Errorf("unused stubs: %d", r)
	}
}
