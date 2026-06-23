package hooks

import (
	"errors"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

var errCredStub = errors.New("stub failure")

var (
	ghCheckArgv = []string{"git", "config", "--get-urlmatch", "credential.helper", "https://github.com"}
	ghLocArgv   = []string{"bash", "-lc", `echo "!$(command -v gh) auth git-credential"`}

	ghResetGithub = []string{"git", "config", "--global", "credential.https://github.com.helper", ""}
	ghSetGithub   = []string{"git", "config", "--global", "credential.https://github.com.helper", "!/usr/bin/gh auth git-credential"}
	ghResetGist   = []string{"git", "config", "--global", "credential.https://gist.github.com.helper", ""}
	ghSetGist     = []string{"git", "config", "--global", "credential.https://gist.github.com.helper", "!/usr/bin/gh auth git-credential"}
	ghCheckArgv2  = []string{"git", "config", "--get-urlmatch", "credential.helper", "https://github.com"}
)

func TestEnsureGHCredentialAuthority_AlreadyHealthy_NoAssertions(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghCheckArgv, "!/usr/bin/gh auth git-credential", nil)
	setRunner(t, fake)

	if err := ensureGHCredentialAuthority(t.Context()); err != nil {
		t.Errorf("ensureGHCredentialAuthority = %v, want nil when already healthy", err)
	}
	if n := fake.Remaining(); n != 0 {
		t.Errorf("unused stubs = %d, want 0", n)
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Errorf("runner calls = %d, want 1 (check only, no reassertions)", len(calls))
	}
}

func TestEnsureGHCredentialAuthority_VSCodeHelper_Reasserts(t *testing.T) {
	vsCodeHelper := `!f() { node /tmp/vscode-remote-containers.js git-credential-helper $*; }; f`
	fake := exec.NewFake().
		Expect(ghCheckArgv, vsCodeHelper, nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghResetGithub, "", nil).
		Expect(ghSetGithub, "", nil).
		Expect(ghResetGist, "", nil).
		Expect(ghSetGist, "", nil).
		Expect(ghCheckArgv2, "!/usr/bin/gh auth git-credential", nil)
	setRunner(t, fake)

	if err := ensureGHCredentialAuthority(t.Context()); err != nil {
		t.Errorf("ensureGHCredentialAuthority = %v, want nil after reassertion", err)
	}
	if n := fake.Remaining(); n != 0 {
		t.Errorf("unused stubs = %d, want 0", n)
	}
}

func TestEnsureGHCredentialAuthority_EmptyHelper_Reasserts(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghCheckArgv, "", nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghResetGithub, "", nil).
		Expect(ghSetGithub, "", nil).
		Expect(ghResetGist, "", nil).
		Expect(ghSetGist, "", nil).
		Expect(ghCheckArgv2, "!/usr/bin/gh auth git-credential", nil)
	setRunner(t, fake)

	if err := ensureGHCredentialAuthority(t.Context()); err != nil {
		t.Errorf("ensureGHCredentialAuthority = %v, want nil after reassertion", err)
	}
}

func TestEnsureGHCredentialAuthority_GHLocateFails_ReturnsError(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghCheckArgv, "", nil).
		Expect(ghLocArgv, "", errCredStub)
	setRunner(t, fake)

	err := ensureGHCredentialAuthority(t.Context())
	if err == nil {
		t.Error("ensureGHCredentialAuthority = nil, want error when gh binary not found")
	}
	if !strings.Contains(err.Error(), "locate gh binary") {
		t.Errorf("error = %q, want to mention 'locate gh binary'", err.Error())
	}
}

func TestEnsureGHCredentialAuthority_PostAssertCheckFails_ReturnsError(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghCheckArgv, "", nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghResetGithub, "", nil).
		Expect(ghSetGithub, "", nil).
		Expect(ghResetGist, "", nil).
		Expect(ghSetGist, "", nil).
		Expect(ghCheckArgv2, `!f() { node /tmp/still-vscode.js $*; }; f`, nil)
	setRunner(t, fake)

	err := ensureGHCredentialAuthority(t.Context())
	if err == nil {
		t.Error("ensureGHCredentialAuthority = nil, want error when post-assert check still shows wrong helper")
	}
	if !strings.Contains(err.Error(), "still does not resolve to gh") {
		t.Errorf("error = %q, want to mention resolver failure", err.Error())
	}
}

func TestEnsureGHCredentialAuthority_Idempotent_HealthyPathNoop(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghCheckArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghCheckArgv, "!/usr/bin/gh auth git-credential", nil)
	setRunner(t, fake)

	for i := range 2 {
		if err := ensureGHCredentialAuthority(t.Context()); err != nil {
			t.Errorf("call %d: ensureGHCredentialAuthority = %v, want nil", i+1, err)
		}
	}
	if n := fake.Remaining(); n != 0 {
		t.Errorf("unused stubs = %d, want 0", n)
	}
}

func TestSessionStart_CredentialAuthorityWarn_OnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MIRABILIS_REPO", t.TempDir())

	fake := exec.NewFake().
		Expect(bashPrefix, "", nil).
		Expect(ghCheckArgv, "", nil).
		Expect(ghLocArgv, "", errCredStub)
	setRunner(t, fake)

	replaceStdin(t, "")
	getOut := captureStdout(t)
	getErr := captureStderr(t)
	err := SessionStart()
	_ = getOut()
	errOut := getErr()

	if err != nil {
		t.Fatalf("SessionStart = %v, want nil (credential failure is warn-only)", err)
	}
	if !strings.Contains(errOut, "credential-authority") {
		t.Errorf("stderr = %q, want WARN mentioning credential-authority", errOut)
	}
}
