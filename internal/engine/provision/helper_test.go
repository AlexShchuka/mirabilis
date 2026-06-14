package provision

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func testDeps(t *testing.T) (Deps, *exec.Fake) {
	t.Helper()
	f := exec.NewFake()
	return Deps{
		Runner: f,
		Cfg:    config.New(t.TempDir()),
		Log:    slog.New(slog.DiscardHandler),
		Repo:   t.TempDir(),
		Home:   t.TempDir(),
	}, f
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteJSON(t *testing.T, path string, m map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, m); err != nil {
		t.Fatal(err)
	}
}

func mustReadJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	m, err := readJSON(path)
	if err != nil {
		t.Fatalf("readJSON(%s): %v", path, err)
	}
	return m
}

func findStep(t *testing.T, steps []pipeline.Command, name string) pipeline.Command {
	t.Helper()
	for _, s := range steps {
		if s.Meta().Name == name {
			return s
		}
	}
	t.Fatalf("step %q not found", name)
	return nil
}

func runStep(t *testing.T, step pipeline.Command) error {
	t.Helper()
	out := make(chan pipeline.Event, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range out {
		}
	}()
	err := step.Run(t.Context(), out, nil)
	close(out)
	<-done
	return err
}

func checkStep(t *testing.T, step pipeline.Command) bool {
	t.Helper()
	ok, err := step.Check(t.Context())
	if err != nil {
		t.Fatalf("check %q: %v", step.Meta().Name, err)
	}
	return ok
}

func testHeadroomBin(d Deps) string {
	return filepath.Join(d.Home, ".headroom-venv/bin/headroom")
}

func headroomScripts(d Deps) map[string]string {
	bin := testHeadroomBin(d)
	return map[string]string{
		"probe": fmt.Sprintf(`test -x %q`, bin),
		"venv":  `python3 -m venv "$HOME/.headroom-venv"`,
		"pip":   `timeout 600 "$HOME/.headroom-venv/bin/pip" install "headroom-ai[mcp,proxy]"`,
		"link":  `mkdir -p "$HOME/.local/bin" && ln -sf "$HOME/.headroom-venv/bin/headroom" "$HOME/.local/bin/headroom"`,
		"curl":  `curl -fsS http://127.0.0.1:8787/stats >/dev/null 2>&1`,
		"start": fmt.Sprintf(`setsid nohup %q proxy --mode cache >"$HOME/.headroom-proxy.log" 2>&1 &`, bin),
		"poll":  `for i in $(seq 1 60); do curl -fsS http://127.0.0.1:8787/stats >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1`,
		"pkill": `pkill -f "headroom proxy" || true`,
		"get":   `claude mcp get headroom`,
		"rm":    `claude mcp remove headroom --scope user >/dev/null 2>&1 || true`,
		"add":   fmt.Sprintf(`claude mcp add --scope user --transport stdio headroom -- %q mcp serve`, bin),
	}
}

func script(s string) []string { return []string{"bash", "-lc", s} }
