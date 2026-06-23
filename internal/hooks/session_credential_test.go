package hooks

import (
	"errors"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

var errCredStub = errors.New("stub failure")

var (
	ghGetAll  = []string{"git", "config", "--get-all", "credential.https://github.com.helper"}
	ghLocArgv = []string{"bash", "-lc", `gh_path=$(command -v gh) && [ -n "$gh_path" ] && echo "!$gh_path auth git-credential"`}

	ghReplaceGithub = []string{"git", "config", "--global", "--replace-all", "credential.https://github.com.helper", ""}
	ghAddGithub     = []string{"git", "config", "--global", "--add", "credential.https://github.com.helper", "!/usr/bin/gh auth git-credential"}
	ghReplaceGist   = []string{"git", "config", "--global", "--replace-all", "credential.https://gist.github.com.helper", ""}
	ghAddGist       = []string{"git", "config", "--global", "--add", "credential.https://gist.github.com.helper", "!/usr/bin/gh auth git-credential"}
)

const healthyGetAll = "\n!/usr/bin/gh auth git-credential"

func TestGhOnlyChain(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want bool
	}{
		{"healthy two-entry", "\n!/usr/bin/gh auth git-credential", true},
		{"healthy trailing newline", "\n!/usr/bin/gh auth git-credential\n", true},
		{"gh only no reset", "!/usr/bin/gh auth git-credential", false},
		{"empty", "", false},
		{"vscode only", `!f() { node /tmp/vscode.js $*; }; f`, false},
		{"gh-only check suffix", "\n!/some/path/gh auth git-credential", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ghOnlyChain(tc.got); got != tc.want {
				t.Errorf("ghOnlyChain(%q) = %v, want %v", tc.got, got, tc.want)
			}
		})
	}
}

func TestEnsureGHCredentialAuthority_AlreadyHealthy_NoAssertions(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghGetAll, healthyGetAll, nil)
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
	vsCodeOnly := `!f() { node /tmp/vscode-remote-containers.js git-credential-helper $*; }; f`
	fake := exec.NewFake().
		Expect(ghGetAll, vsCodeOnly, nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghReplaceGithub, "", nil).
		Expect(ghAddGithub, "", nil).
		Expect(ghReplaceGist, "", nil).
		Expect(ghAddGist, "", nil).
		Expect(ghGetAll, healthyGetAll, nil)
	setRunner(t, fake)

	if err := ensureGHCredentialAuthority(t.Context()); err != nil {
		t.Errorf("ensureGHCredentialAuthority = %v, want nil after reassertion", err)
	}
	if n := fake.Remaining(); n != 0 {
		t.Errorf("unused stubs = %d, want 0", n)
	}
}

func TestEnsureGHCredentialAuthority_GhWithoutReset_Reasserts(t *testing.T) {
	ghNoReset := "!/usr/bin/gh auth git-credential"
	fake := exec.NewFake().
		Expect(ghGetAll, ghNoReset, nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghReplaceGithub, "", nil).
		Expect(ghAddGithub, "", nil).
		Expect(ghReplaceGist, "", nil).
		Expect(ghAddGist, "", nil).
		Expect(ghGetAll, healthyGetAll, nil)
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
		Expect(ghGetAll, "", nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghReplaceGithub, "", nil).
		Expect(ghAddGithub, "", nil).
		Expect(ghReplaceGist, "", nil).
		Expect(ghAddGist, "", nil).
		Expect(ghGetAll, healthyGetAll, nil)
	setRunner(t, fake)

	if err := ensureGHCredentialAuthority(t.Context()); err != nil {
		t.Errorf("ensureGHCredentialAuthority = %v, want nil after reassertion", err)
	}
}

func TestEnsureGHCredentialAuthority_GHLocateFails_ReturnsError(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghGetAll, "", nil).
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
		Expect(ghGetAll, "", nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghReplaceGithub, "", nil).
		Expect(ghAddGithub, "", nil).
		Expect(ghReplaceGist, "", nil).
		Expect(ghAddGist, "", nil).
		Expect(ghGetAll, `!/usr/bin/gh auth git-credential`, nil)
	setRunner(t, fake)

	err := ensureGHCredentialAuthority(t.Context())
	if err == nil {
		t.Error("ensureGHCredentialAuthority = nil, want error when post-assert chain is not gh-only")
	}
	if !strings.Contains(err.Error(), "still does not resolve to gh-only") {
		t.Errorf("error = %q, want to mention resolver failure", err.Error())
	}
}

func TestEnsureGHCredentialAuthority_Idempotent_HealthyPathNoop(t *testing.T) {
	fake := exec.NewFake().
		Expect(ghGetAll, healthyGetAll, nil).
		Expect(ghGetAll, healthyGetAll, nil)
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

func TestEnsureGHCredentialAuthority_GlobalVSCodeHelperNoURLScope_Reasserts(t *testing.T) {
	vsCodeHelper := `!f() { node /tmp/vscode-remote-containers.js $*; }; f`
	fake := exec.NewFake().
		Expect(ghGetAll, vsCodeHelper, nil).
		Expect(ghLocArgv, "!/usr/bin/gh auth git-credential", nil).
		Expect(ghReplaceGithub, "", nil).
		Expect(ghAddGithub, "", nil).
		Expect(ghReplaceGist, "", nil).
		Expect(ghAddGist, "", nil).
		Expect(ghGetAll, healthyGetAll, nil)
	setRunner(t, fake)

	if err := ensureGHCredentialAuthority(t.Context()); err != nil {
		t.Errorf("ensureGHCredentialAuthority = %v, want nil; global VS Code helper must trigger reassertion", err)
	}
	calls := fake.Calls()
	hasReplaceAll := false
	for _, c := range calls {
		for _, a := range c.Argv {
			if a == "--replace-all" {
				hasReplaceAll = true
			}
		}
	}
	if !hasReplaceAll {
		t.Error("expected --replace-all in reassertion calls to establish reset+gh two-entry layout")
	}
}

func TestSessionStart_CredentialAuthorityWarn_OnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MIRABILIS_REPO", t.TempDir())

	fake := exec.NewFake().
		Expect(bashPrefix, "", nil).
		Expect(ghGetAll, "", nil).
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
