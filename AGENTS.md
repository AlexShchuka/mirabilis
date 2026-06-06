# AGENTS.md

Canonical agent guide for **mirabilis** — an autonomous Claude Code workspace.
Tool-neutral by design; `CLAUDE.md` is a symlink to this file.

## What this repo is

mirabilis is a reproducible, isolated dev-container workspace on macOS where an
autonomous Claude Code agent develops repositories. It is built on Anthropic's
official dev-container **Feature** for the Claude Code CLI (consumed, not
vendored), with a thin mirabilis layer on top: the
[`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) plugin preinstalled
via a self-owned marketplace, MCP servers, and a configurable egress allowlist.

## Golden rules for any agent working here

1. **No comments in code or config.** Code describes itself. All prose belongs
   in Markdown docs. This applies to shell, Dockerfile, Makefile, JSON, YAML,
   `.env` — everything except `.md` files.
2. **Never commit secrets.** Tokens live in the macOS Keychain (`scripts/token.sh`)
   and are injected as environment variables at run time. `.env` and token files
   are gitignored. If you ever see a token in a diff, stop.
3. **Branch, never push to `main`.** All work lands via a **squash-merged PR**
   into `main`; the source branch is deleted after merge.
4. **Work inside `/workspace`.** That bind-mount is the single source of truth,
   shared with the host IDE. Avoid editing a file the human is editing; prefer a
   git worktree or your own branch.
5. **Egress is a configurable allowlist.** New outbound hosts go in
   `config/settings.json` under `sandbox.network.allowedDomains`, deliberately —
   not in an iptables script.

## Commands

Onboarding is two commands — `git clone`, then `./mirabilis`. The first run
self-bootstraps (installs Docker Desktop + the devcontainer CLI if missing), puts
`mirabilis` on PATH, builds the container, and configures tokens/login. After
that, daily use is the single `mirabilis` command:

```
mirabilis                  start the workspace and open Claude (first run self-configures)
mirabilis update           pull the latest version and rebuild (memory + auth kept)
mirabilis agent "<task>"   run one prompt non-interactively
mirabilis shell            a shell in the workspace as node
mirabilis doctor           health check
mirabilis down | restart   stop / recreate the workspace
```

On launch `mirabilis` warns (but still starts) when the checkout is behind the
remote or the container is behind the checkout: the image is stamped with the
source git revision (`MIRABILIS_VERSION`, threaded build arg → env) so staleness
is detectable, and `mirabilis update` rebuilds. The `make` targets still cover
install/lifecycle for power users / CI.

## How it fits together

- **Engine:** a `.devcontainer` driven by the `@devcontainers/cli`. The image
  comes from `docker/Dockerfile` (minimal: node base + bubblewrap/socat + a
  marketplace seed), and the Claude Code CLI + `gh` are added as official
  **Features**. `docker-compose.yml` provides the volumes and the `/workspace`
  mount. `.devcontainer/refresh.sh` runs on **every start** (`postStartCommand`):
  it re-seeds settings, refreshes the plugin (`claude plugin update`), and
  registers MCP — so re-running `mirabilis` **updates** the sandbox in place
  rather than from scratch, while the persistent volumes keep memory and auth.
- **Filesystem:** persistent (`~/.claude` auth+memory+plugins, `~/.config/gh`,
  `/workspace`) survives rebuilds; ephemeral (`/tmp`, the refreshed plugin) is
  renewed each start.
- **System prompt:** layered and deliberately thin — neuro-matrix injects its
  protocol via a SessionStart hook, and `mirabilis` appends a short sandbox note
  (`config/sandbox-context.md`) via `--append-system-prompt-file`. Both stack;
  neither replaces the other.
- **Egress:** Claude Code's native sandbox confines the agent's Bash commands to
  `sandbox.network.allowedDomains` (no iptables, no `NET_ADMIN`). Geo-exit is the
  host VPN. WebSearch/WebFetch ride the Anthropic API, so the allowlist never
  blocks them.
- **Plugin:** `.claude-plugin/marketplace.json` defines the `mirabilis`
  marketplace, which installs `neuro-matrix` at user scope. A plugin's root
  `CLAUDE.md` is **not** loaded as context — neuro-matrix injects its protocol
  via a SessionStart hook.
- **MCP:** GitHub (hosted HTTP) and Context7 are registered in
  `scripts/provision-mcp.sh`; tokens are read from the environment.

See `README.md` for setup.
