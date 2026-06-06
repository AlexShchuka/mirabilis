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
2. **Never commit secrets.** GitHub and Claude sign-in are native (`gh auth login`,
   Claude's first-run login) and persist in the sandbox volumes; the Context7 key
   lives in the macOS Keychain (`scripts/token.sh`). `.env` and token files are
   gitignored. If you ever see a token in a diff, stop.
3. **Branch, never push to `main`.** All work lands via a **squash-merged PR**
   into `main`; the source branch is deleted after merge.
4. **Work inside `/workspace`.** That bind-mount is the single source of truth,
   shared with the host IDE. Avoid editing a file the human is editing; prefer a
   git worktree or your own branch.
5. **Egress is a configurable allowlist.** New outbound hosts go in
   `config/settings.json` under `sandbox.network.allowedDomains`, deliberately —
   not in an iptables script.

## Commands

Onboarding is one line — `git clone … && mirabilis/mirabilis` (no `cd`). The first
run self-bootstraps (installs Docker Desktop + the devcontainer CLI if missing),
installs `mirabilis` on PATH globally, builds the container, and signs you in to
GitHub and Claude (native flows, saved in the volumes). After that, run
`mirabilis` from anywhere:

```
mirabilis                  start the workspace and open Claude (first run self-configures)
mirabilis update           pull the latest version and rebuild (memory + auth kept)
mirabilis doctor           health check
mirabilis down | restart   stop / recreate the workspace
```

Launching is idempotent. When the container's version matches your checkout it is
reused as-is — `mirabilis` just starts it and opens Claude, nothing is recreated.
When the container is **behind your checkout** it asks `rebuild it now? [y/N]`:
yes rebuilds in place, no keeps the existing container. When your **checkout is
behind the remote** it notes a newer version is available (`mirabilis update`).
Staleness is detectable because the image is stamped with the source git revision
(`MIRABILIS_VERSION`, threaded build arg → env), read back via `docker inspect`
(no `exec`, so the check works even on a wedged container). The `make` targets
still cover install/lifecycle for power users / CI.

## How it fits together

- **Engine:** a `.devcontainer` driven by the `@devcontainers/cli`. The image
  comes from `docker/Dockerfile` (minimal: node base + bubblewrap/socat + a
  marketplace seed), and the Claude Code CLI + `gh` are added as official
  **Features**. `docker-compose.yml` provides the volumes and the `/workspace`
  mount. `.devcontainer/refresh.sh` runs on **every start** (`postStartCommand`):
  it re-seeds settings, refreshes the plugin (`claude plugin update`), and
  registers MCP — so re-running `mirabilis` **updates** the sandbox in place
  rather than from scratch, while the persistent volumes keep memory and auth.
- **Filesystem (three tiers):** persistent **volumes** inside the sandbox —
  `~/.claude` (memory + Claude credentials + plugins) and `~/.config/gh` (GitHub
  credentials) — survive `mirabilis update`; the **workspace** `/workspace`
  (repos + code) is bind-mounted from the host and shared with the editor;
  everything else, including `/tmp`, is **ephemeral** and wiped on restart/update.
- **System prompt:** at launch `mirabilis` builds the append-system-prompt file
  by concatenating `config/sandbox-context.md` with the neuro-matrix plugin's
  `CLAUDE.md` (the agent protocol), located by globbing the versioned plugin
  cache; if the plugin file is absent it falls back to the sandbox note and
  warns. It is passed via `--append-system-prompt-file`. (neuro-matrix's hooks
  add invariant and verification gates — they do not inject the protocol.)
- **Egress:** Claude Code's native sandbox confines the agent's Bash commands to
  `sandbox.network.allowedDomains` (no iptables, no `NET_ADMIN`). WebSearch and
  WebFetch ride the Anthropic API, so the allowlist never blocks them.
- **Plugin:** `.claude-plugin/marketplace.json` defines the `mirabilis`
  marketplace, which installs `neuro-matrix` at user scope. Claude Code does not
  auto-load a plugin's `CLAUDE.md`, so `mirabilis` appends it to the system
  prompt itself (see **System prompt**); the plugin's hooks add the invariant and
  verification gates.
- **MCP:** GitHub (hosted HTTP) and Context7 are registered in
  `scripts/provision-mcp.sh`; the GitHub token comes from your `gh` login
  (`gh auth token`) and the Context7 key from the environment.

See `README.md` for setup.
