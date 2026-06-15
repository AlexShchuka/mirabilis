package provision

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/harness"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

var errStub = errors.New("stub failure")

var contractPrep = map[string]func(t *testing.T, d *Deps, f *exec.Fake){
	"settings": func(t *testing.T, d *Deps, _ *exec.Fake) {
		mustWriteJSON(t, d.Cfg.SettingsSeed(), map[string]any{
			"hooks":      map[string]any{"PreToolUse": []any{}},
			"statusLine": "seed",
			"env":        map[string]any{"FOO": "1"},
		})
	},
	"onboarding": func(_ *testing.T, _ *Deps, _ *exec.Fake) {},
	"theme": func(t *testing.T, d *Deps, _ *exec.Fake) {
		mustWriteJSON(t, d.settingsPath(), map[string]any{})
		mustWrite(t, d.themePath(), "dark\n")
	},
	"memory": func(t *testing.T, d *Deps, _ *exec.Fake) {
		mustWrite(t, filepath.Join(d.Repo, ".mirabilis", "saved-memory", "about-me.md"), "restored\n")
	},
	"git-identity": func(_ *testing.T, _ *Deps, f *exec.Fake) {
		f.Expect([]string{"gh", "auth", "status"}, "", nil)
		f.Expect([]string{"git", "version"}, "", nil)
		f.Expect([]string{"git", "config", "--global", "user.name"}, "", errStub)
		f.Expect([]string{"gh", "auth", "status"}, "", nil)
		f.Expect([]string{"git", "version"}, "", nil)
		f.Expect([]string{"gh", "api", "user"}, `{"login":"alex","name":"Alex","email":"a@b.c","id":7}`, nil)
		f.Expect([]string{"git", "config", "--global", "user.name", "Alex"}, "", nil)
		f.Expect([]string{"git", "config", "--global", "user.email", "a@b.c"}, "", nil)
		f.Expect([]string{"gh", "auth", "setup-git"}, "", nil)
		f.Expect([]string{"gh", "auth", "status"}, "", nil)
		f.Expect([]string{"git", "version"}, "", nil)
		f.Expect([]string{"git", "config", "--global", "user.name"}, "Alex", nil)
		f.Expect([]string{"git", "config", "--global", "user.email"}, "a@b.c", nil)
	},
	"claude-hud": func(t *testing.T, d *Deps, _ *exec.Fake) {
		mustWrite(t, d.Cfg.HudConfigSeed(), "{}\n")
	},
	"cav-shrink": func(t *testing.T, d *Deps, f *exec.Fake) {
		mustWrite(t, filepath.Join(d.Repo, "config", "mcp.json"),
			`[{"name":"seq","transport":"stdio","args":["npx","-y","@mcp/seq"],"shrink":true}]`)
		f.Expect([]string{"npx", "caveman-shrink", "--with-mcp-shrink=npx -y @mcp/seq"}, "", nil)
	},
	"mcp": func(t *testing.T, d *Deps, f *exec.Fake) {
		mustWrite(t, filepath.Join(d.Repo, "config", "mcp.json"),
			`[{"name":"context7","transport":"http","url":"https://mcp.context7.com/mcp"},`+
				`{"name":"sequential-thinking","transport":"stdio","args":["npx","-y","@modelcontextprotocol/server-sequential-thinking"]},`+
				`{"name":"arxiv-mcp-server","transport":"stdio","args":["uvx","arxiv-mcp-server"]},`+
				`{"name":"docling","transport":"stdio","args":["uvx","--from","docling-mcp[local]","docling-mcp-server","--transport","stdio"]}]`)
		cv := script("command -v claude")
		uv := script("command -v uvx")
		list := []string{"claude", "mcp", "list"}
		f.Expect(cv, "", nil)
		f.Expect(uv, "", nil)
		f.Expect(list, "", nil)
		f.Expect(cv, "", nil)
		f.Expect(uv, "", nil)
		f.Expect(list, "", nil)
		f.Expect([]string{"claude", "mcp", "add", "--scope", "user", "--transport", "http", "context7", "https://mcp.context7.com/mcp"}, "", nil)
		f.Expect([]string{"claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "sequential-thinking", "--", "npx", "-y", "@modelcontextprotocol/server-sequential-thinking"}, "", nil)
		f.Expect([]string{"claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "arxiv-mcp-server", "--", "uvx", "arxiv-mcp-server"}, "", nil)
		f.Expect([]string{"claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "docling", "--", "uvx", "--from", "docling-mcp[local]", "docling-mcp-server", "--transport", "stdio"}, "", nil)
		f.Expect(cv, "", nil)
		f.Expect(uv, "", nil)
		f.Expect(list, "context7  http\nsequential-thinking  stdio\narxiv-mcp-server  stdio\ndocling  stdio", nil)
	},
	"rtk": func(t *testing.T, d *Deps, f *exec.Fake) {
		mustWriteJSON(t, d.settingsPath(), map[string]any{
			"hooks": map[string]any{
				"PreToolUse": []any{
					map[string]any{"hooks": []any{map[string]any{"command": "rtk hook claude"}}},
				},
			},
		})
		f.Expect([]string{"rtk", "--version"}, "rtk 0.1", nil)
		f.Expect([]string{"rtk", "--version"}, "rtk 0.1", nil)
		f.Expect([]string{"rtk", "--version"}, "rtk 0.1", nil)
	},
	"rtk-config": func(t *testing.T, d *Deps, _ *exec.Fake) {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(d.Home, "xdg"))
		mustWrite(t, d.Cfg.RTKConfigSeed(), "[rtk]\n")
	},
	"harness": func(_ *testing.T, _ *Deps, f *exec.Fake) {
		probe := script(harness.ProbeScript)
		f.Expect(probe, "", errStub)
		f.Expect(probe, "", errStub)
		f.Expect(script("command -v claude"), "", nil)
		f.Expect([]string{"claude", "plugin", "marketplace", "add", "AlexShchuka/neuro-matrix"}, "", nil)
		f.Expect([]string{"claude", "plugin", "install", "neuro-matrix@neuro-matrix", "--scope", "user"}, "", nil)
		f.Expect([]string{"claude", "plugin", "update", "neuro-matrix@neuro-matrix"}, "", nil)
		f.Expect(probe, "", nil)
		f.Expect(script(harness.RelinkScript), "", nil)
		f.Expect(probe, "", nil)
	},
	"plugins": func(t *testing.T, d *Deps, f *exec.Fake) {
		mustWrite(t, filepath.Join(d.Repo, "config", "marketplaces.txt"), "anthropics/claude-plugins-official\njarrodwatts/claude-hud\n")
		mustWrite(t, d.Cfg.PluginsTxt(), "alpha@1.0\nbeta\n")
		mustWrite(t, filepath.Join(d.claudeDir(), filePluginsDisabled), "beta\n")
		mustWriteJSON(t, d.settingsPath(), map[string]any{})
		cv := script("command -v claude")
		list := []string{"claude", "plugin", "list"}
		f.Expect(cv, "", nil)
		f.Expect(list, "", nil)
		f.Expect(cv, "", nil)
		f.Expect(script(`mkdir -p "$HOME/.cache/tmp"`), "", nil)
		f.Expect([]string{"claude", "plugin", "marketplace", "add", "anthropics/claude-plugins-official"}, "", nil)
		f.Expect([]string{"claude", "plugin", "marketplace", "add", "jarrodwatts/claude-hud"}, "", nil)
		f.Expect(list, "", nil)
		f.Expect(script(`TMPDIR="$HOME/.cache/tmp" claude plugin install "alpha@1.0" --scope user`), "", nil)
		f.Expect(cv, "", nil)
		f.Expect(list, "alpha 1.0 enabled", nil)
	},
	"skills": func(t *testing.T, d *Deps, f *exec.Fake) {
		mustWrite(t, d.Cfg.SkillsTxt(), "owner/repo-a\n")
		mustWrite(t, filepath.Join(d.claudeDir(), fileSkills), "owner/repo-a\n")
		dir := filepath.Join(d.claudeDir(), "skills", "repo-a")
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		gv := []string{"git", "version"}
		f.Expect(gv, "", nil)
		f.Expect([]string{"git", "-C", dir, "pull", "--ff-only"}, "", nil)
	},
	"headroom": func(t *testing.T, d *Deps, f *exec.Fake) {
		d.ProxyAddr = "http://host.docker.internal:8788"
		if err := os.MkdirAll(d.claudeDir(), 0o755); err != nil {
			t.Fatal(err)
		}
		sc := headroomScripts(*d)
		f.Expect(script(sc["probe"]), "", errStub)
		f.Expect(script(sc["probe"]), "", errStub)
		f.Expect(script(sc["venv"]), "", nil)
		f.Expect(script(sc["pip"]), "", nil)
		f.Expect(script(sc["link"]), "", nil)
		f.Expect(script(sc["curl"]), "", errStub)
		f.Expect(script(sc["curl"]), "", errStub)
		f.Expect(script(sc["start"]), "", nil)
		f.Expect(script(sc["poll"]), "", nil)
		f.Expect(script(sc["get"]), "", errStub)
		f.Expect(script(sc["rm"]), "", nil)
		f.Expect(script(sc["add"]), "", nil)
		f.Expect(script(sc["probe"]), "", nil)
		f.Expect(script(sc["curl"]), "", nil)
		f.Expect(script(sc["get"]), ".headroom-venv/bin/headroom", nil)
	},
	"local-offload": func(t *testing.T, _ *Deps, f *exec.Fake) {
		self, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable: %v", err)
		}
		cv := script("command -v claude")
		get := []string{"claude", "mcp", "get", "local-offload"}
		add := []string{"claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "local-offload"}
		f.Expect(cv, "", nil)
		f.Expect(get, "", errors.New("not registered"))
		f.Expect(cv, "", nil)
		f.Expect(get, "", errors.New("not registered"))
		f.Expect(add, "", nil)
		f.Expect(cv, "", nil)
		f.Expect(get, "local-offload stdio "+self+" localllm serve", nil)
	},
	"settings-env": func(_ *testing.T, d *Deps, _ *exec.Fake) {
		d.SessionKey = "sk-contract"
	},
	"create-marker":      func(_ *testing.T, _ *Deps, _ *exec.Fake) {},
	"claude-credentials": func(_ *testing.T, _ *Deps, _ *exec.Fake) {},
	"start-marker": func(_ *testing.T, d *Deps, _ *exec.Fake) {
		d.Fingerprint = "vtest"
		d.SessionKey = "sk-contract"
	},
}

func TestContractRegistries(t *testing.T) {
	registries := []struct {
		build func(Deps) []pipeline.Command
		name  string
	}{
		{name: "create", build: Create},
		{name: "start", build: Start},
		{name: "plugins", build: Plugins},
		{name: "skills", build: Skills},
	}
	for _, reg := range registries {
		t.Run(reg.name, func(t *testing.T) {
			for _, proto := range reg.build(Deps{}) {
				name := proto.Meta().Name
				t.Run(name, func(t *testing.T) {
					prep, ok := contractPrep[name]
					if !ok {
						t.Fatalf("no contract prep for step %q", name)
					}
					d, f := testDeps(t)
					prep(t, &d, f)
					step := findStep(t, reg.build(d), name)
					pipeline.Contract(t, step, nil)
					if n := f.Remaining(); n != 0 {
						t.Errorf("unused stubs after contract: %d", n)
					}
				})
			}
		})
	}
}
