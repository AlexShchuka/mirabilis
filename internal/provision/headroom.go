package provision

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

const HeadroomProxyURL = "http://127.0.0.1:8787"
const HeadroomBaseURLKey = "ANTHROPIC_BASE_URL"

func EnsureHeadroom(ctx context.Context, r runner.Runner) error {
	if _, err := r.Container(ctx, "bash", "-lc", `test -x "$HOME/.headroom-venv/bin/headroom"`); err != nil {
		if _, err := r.Container(ctx, "bash", "-lc", `python3 -m venv "$HOME/.headroom-venv"`); err != nil {
			warn("headroom venv", err)
			return nil
		}
		if _, err := r.Container(ctx, "bash", "-lc", `timeout 600 "$HOME/.headroom-venv/bin/pip" install "headroom-ai[mcp,proxy]"`); err != nil {
			warn("headroom pip install", err)
			return nil
		}
		if _, err := r.Container(ctx, "bash", "-lc", `mkdir -p "$HOME/.local/bin" && ln -sf "$HOME/.headroom-venv/bin/headroom" "$HOME/.local/bin/headroom"`); err != nil {
			warn("headroom symlink", err)
			return nil
		}
	}

	if out, err := r.Container(ctx, "bash", "-lc", "claude mcp get headroom"); err == nil && strings.Contains(out, ".headroom-venv/bin/headroom") {
		return nil
	}

	_, _ = r.Container(ctx, "claude", "mcp", "remove", "headroom", "--scope", "user")
	if _, err := r.Container(ctx, "bash", "-lc", `claude mcp add --scope user --transport stdio headroom -- "$HOME/.headroom-venv/bin/headroom" mcp serve`); err != nil {
		warn("headroom mcp register", err)
	}
	return nil
}

func EnsureHeadroomProxy(ctx context.Context, r runner.Runner) error {
	if !proxyReachable(ctx, r) {
		startProxy(ctx, r)
		if !pollProxy(ctx, r, 60) {
			warn("headroom proxy start", os.ErrProcessDone)
			removeBaseURL()
			return nil
		}
	}
	setBaseURL()
	return nil
}

func proxyReachable(ctx context.Context, r runner.Runner) bool {
	_, err := r.Container(ctx, "bash", "-lc", "curl -fsS "+HeadroomProxyURL+"/stats >/dev/null 2>&1")
	return err == nil
}

func startProxy(ctx context.Context, r runner.Runner) {
	_, _ = r.Container(ctx, "bash", "-lc",
		`setsid nohup "$HOME/.headroom-venv/bin/headroom" proxy >"$HOME/.headroom-proxy.log" 2>&1 &`)
}

func pollProxy(ctx context.Context, r runner.Runner, maxAttempts int) bool {
	script := fmt.Sprintf(`for i in $(seq 1 %d); do curl -fsS `+HeadroomProxyURL+`/stats >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1`, maxAttempts)
	_, err := r.Container(ctx, "bash", "-lc", script)
	return err == nil
}

func setBaseURL() {
	dest := settingsPath()
	m, err := readJSON(dest)
	if err != nil {
		warn("headroom proxy read settings", err)
		return
	}
	env, _ := m["env"].(map[string]any)
	if env == nil {
		env = make(map[string]any)
	}
	if env[HeadroomBaseURLKey] == HeadroomProxyURL {
		return
	}
	env[HeadroomBaseURLKey] = HeadroomProxyURL
	m["env"] = env
	warn("headroom proxy write settings", WriteJSON(dest, m))
}

func removeBaseURL() {
	dest := settingsPath()
	m, err := readJSON(dest)
	if err != nil {
		return
	}
	env, _ := m["env"].(map[string]any)
	if env == nil || env[HeadroomBaseURLKey] == nil {
		return
	}
	delete(env, HeadroomBaseURLKey)
	m["env"] = env
	warn("headroom proxy remove base url", WriteJSON(dest, m))
}
