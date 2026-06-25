package provision

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
)

func stubGitIdentityProbes(f *exec.Fake, name, email string) {
	f.Expect([]string{"gh", "auth", "status"}, "", nil)
	f.Expect([]string{"git", "version"}, "", nil)
	if name == "" {
		f.Expect([]string{"git", "config", "--global", "user.name"}, "", errors.New("unset"))
		return
	}
	f.Expect([]string{"git", "config", "--global", "user.name"}, name, nil)
	f.Expect([]string{"git", "config", "--global", "user.email"}, email, nil)
}

func stubMCPProbes(f *exec.Fake, list string) {
	f.Expect(script("command -v claude"), "", nil)
	f.Expect(script("command -v uvx"), "", errors.New("missing"))
	f.Expect([]string{"claude", "mcp", "list"}, list, nil)
}

func TestRunPhaseCreateIsIdempotent(t *testing.T) {
	d, f := testDeps(t)
	mustWrite(t, filepath.Join(d.Repo, "config", "mcp.json"),
		`[{"name":"context7","transport":"http","url":"https://mcp.context7.com/mcp"},`+
			`{"name":"sequential-thinking","transport":"stdio","args":["npx","-y","@modelcontextprotocol/server-sequential-thinking"]},`+
			`{"name":"arxiv-mcp-server","transport":"stdio","args":["uvx","arxiv-mcp-server"]},`+
			`{"name":"docling","transport":"stdio","args":["uvx","--from","docling-mcp[local]","docling-mcp-server","--transport","stdio"]}]`)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(d.Home, "xdg"))
	mustWriteJSON(t, d.Cfg.SettingsSeed(), map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"hooks": []any{map[string]any{"command": "rtk hook claude"}}},
			},
		},
		"statusLine": "seed",
		"env":        map[string]any{"FOO": "1"},
	})
	mustWrite(t, d.Cfg.HudConfigSeed(), "{}\n")
	mustWrite(t, d.Cfg.RTKConfigSeed(), "[rtk]\n")
	mustWrite(t, filepath.Join(d.Repo, ".mirabilis", "saved-memory", "about-me.md"), "saved\n")

	stubGitIdentityProbes(f, "", "")
	f.Expect([]string{"gh", "auth", "status"}, "", nil)
	f.Expect([]string{"git", "version"}, "", nil)
	f.Expect([]string{"gh", "api", "user"}, `{"login":"alex","name":"Alex","email":"a@b.c","id":7}`, nil)
	f.Expect([]string{"git", "config", "--global", "user.name", "Alex"}, "", nil)
	f.Expect([]string{"git", "config", "--global", "user.email", "a@b.c"}, "", nil)
	f.Expect([]string{"gh", "auth", "setup-git"}, "", nil)
	stubMCPProbes(f, "")
	stubMCPProbes(f, "")
	f.Expect([]string{"claude", "mcp", "add", "--scope", "user", "--transport", "http", "context7", "https://mcp.context7.com/mcp"}, "", nil)
	f.Expect([]string{"claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "sequential-thinking", "--", "npx", "-y", "@modelcontextprotocol/server-sequential-thinking"}, "", nil)
	f.Expect([]string{"rtk", "--version"}, "rtk 0.1", nil)

	if err := RunPhase(t.Context(), d, "create"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Fatalf("first create left %d unused stubs", n)
	}
	marker, err := os.ReadFile(filepath.Join(d.claudeDir(), harness.CreateMarkerName))
	if err != nil {
		t.Fatal(err)
	}
	if string(marker) != "ok\n" {
		t.Errorf("create marker = %q, want ok", marker)
	}

	probes := exec.NewFake()
	d.Runner = probes
	stubGitIdentityProbes(probes, "Alex", "a@b.c")
	stubMCPProbes(probes, "context7  http\nsequential-thinking  stdio")
	probes.Expect([]string{"rtk", "--version"}, "rtk 0.1", nil)

	if err := RunPhase(t.Context(), d, "create"); err != nil {
		t.Fatalf("second create: %v", err)
	}
	if got := len(probes.Calls()); got != 8 {
		t.Errorf("second create made %d runner calls, want 8 probes only: %v", got, probes.Calls())
	}
	if n := probes.Remaining(); n != 0 {
		t.Errorf("second create left %d unused stubs", n)
	}
}

func TestRunPhaseUnknownPhase(t *testing.T) {
	d, _ := testDeps(t)
	if err := RunPhase(t.Context(), d, "bogus"); err == nil {
		t.Error("unknown phase must error")
	}
}

func TestUpdateIncludesHarnessRefresh(t *testing.T) {
	d, _ := testDeps(t)
	var names []string
	for _, c := range Update(d) {
		names = append(names, c.Meta().Name)
	}
	wantEcosystem, wantHarness := false, false
	for _, n := range names {
		switch n {
		case "ecosystem":
			wantEcosystem = true
		case "harness":
			wantHarness = true
		}
	}
	if !wantEcosystem {
		t.Errorf("Update missing ecosystem fetch step: %v", names)
	}
	if !wantHarness {
		t.Errorf("Update missing harness step: a menu UPDATE must refresh the live plugin, not just source: %v", names)
	}
}

func TestRunPhaseUpdateRefreshesLiveHarness(t *testing.T) {
	d, f := testDeps(t)

	root := filepath.Join(d.Home, ecosystemDirRel)
	f.Expect([]string{"gh", "auth", "status"}, "", nil)
	for _, name := range ecosystemRepos {
		dir := filepath.Join(root, name)
		mustWrite(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")
		f.Expect(script(fmt.Sprintf(`test -d %q`, filepath.Join(dir, ".git"))), "", nil)
	}

	probe := script(harness.ProbeScript)
	f.Expect(probe, "", errStub)
	f.Expect(probe, "", errStub)
	f.Expect(script("command -v claude"), "", nil)
	f.Expect([]string{"claude", "plugin", "marketplace", "add", "AlexShchuka/neuro-matrix"}, "", nil)
	f.Expect([]string{"claude", "plugin", "install", "neuro-matrix@neuro-matrix", "--scope", "user"}, "", nil)
	f.Expect([]string{"claude", "plugin", "update", "neuro-matrix@neuro-matrix"}, "", nil)
	f.Expect(probe, "", nil)
	f.Expect(script(harness.RelinkScript), "", nil)

	if err := RunPhase(t.Context(), d, "update"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("update left %d unused stubs", n)
	}

	want := map[string]bool{
		"claude plugin install neuro-matrix@neuro-matrix --scope user": false,
		"claude plugin update neuro-matrix@neuro-matrix":               false,
	}
	relinked := false
	for _, c := range f.Calls() {
		joined := strings.Join(c.Argv, " ")
		if _, ok := want[joined]; ok {
			want[joined] = true
		}
		if len(c.Argv) >= 3 && c.Argv[2] == harness.RelinkScript {
			relinked = true
		}
	}
	for action, seen := range want {
		if !seen {
			t.Errorf("update did not refresh live plugin: missing %q", action)
		}
	}
	if !relinked {
		t.Error("update did not relink ~/.neuro-matrix to the refreshed plugin")
	}
}
