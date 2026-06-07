# AGENTS.md

Agent guide for **mirabilis** — an autonomous Claude Code workspace: an isolated,
reproducible dev container on macOS where an agent develops repositories with the
[`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) harness preinstalled.
`CLAUDE.md` is a symlink to this file. See `README.md` for setup and `SECURITY.md`
for the threat model.

## Golden rules

1. **No comments in code or config.** Code describes itself; prose lives in
   Markdown. Applies to shell, Dockerfile, Makefile, JSON, YAML, `.env` —
   everything except `.md` files.
2. **Never commit secrets.** Sign-in is native (`gh auth login`, Claude's
   first-run login) and persists in the sandbox volumes; the Context7 key lives in
   the macOS Keychain (`src/token.sh`). If a token ever appears in a diff, stop.
3. **Branch, never push to `main`.** Work lands via one squash-merged PR per task;
   the source branch is deleted after merge.
4. **Work inside `/workspace`.** The single source of truth — a named volume the
   sandbox owns, opened from your editor via VSCode (Dev Containers attach), not a
   host folder. Prefer your own branch or a git worktree over editing a file in use.
5. **Egress is a configurable allowlist.** New outbound hosts go in
   `config/settings.json` → `sandbox.network.allowedDomains`, deliberately.

## Commands

```
mirabilis            the only command — opens a menu: launch / update / plugins / harness / sign-in / theme
```

Every run opens the menu, highlighting anything stale (the workspace container, the
mirabilis repo, the neuro-matrix harness); pick **launch** to drop into Claude. Update
and sign-in live in the menu, not as subcommands. `mirabilis completion zsh` and
`mirabilis version` are the only other invocations. The launcher is `src/bin/mirabilis`,
a thin entrypoint that sources single-responsibility modules in `src/lib/` (util,
version, proxy, docker, preflight, auth, prompt, menu) — read the relevant `src/lib/*.sh`
before changing that behaviour.

## Where things live

Change a behaviour in its owning file, not by restating it here.

| Concern | File |
|---|---|
| Launcher entrypoint: bash re-exec, config consts, sources `src/lib/`, dispatches | `src/bin/mirabilis` |
| Launcher modules, one concern each: shared utils · versioning/staleness · egress proxy · docker lifecycle · preflight + fail-fast gate · auth/theme · system-prompt assembly · menu | `src/lib/*.sh` |
| Image: base, system tools, Python + .NET SDK, RTK binary (pinned) | `docker/Dockerfile` |
| Volumes, `/workspace` mount, container env, proxy wiring, entrypoint | `docker-compose.yml` |
| Dev-container engine; Claude Code CLI + `gh` as official Features | `.devcontainer/devcontainer.json` |
| Container entrypoint: runs per-start setup on every start (incl. auto-restart) | `.devcontainer/entrypoint.sh` |
| Per-start setup (idempotent): settings seed, theme, trust, git identity (from gh), harness (opt-out) + plugins from catalog, MCP, apt-list | `.devcontainer/refresh.sh` |
| Declared apt packages (re-applied at start) | `config/apt-packages.txt` |
| Plugin catalog (installed at start unless deselected at first run) | `config/plugins.txt` |
| Claude settings: sandbox allowlist + filesystem, plugins, theme | `config/settings.json` |
| Agent-facing sandbox note (prepended to the system prompt) | `config/sandbox-context.md` |
| Plugin marketplace (installs neuro-matrix, pinned `ref`) | `.claude-plugin/marketplace.json` |
| MCP registration (Context7) | `src/provision-mcp.sh` |
| Secret storage (macOS Keychain ↔ env) | `src/token.sh` |
| Prerequisites / install / clean (power users, CI) | `Makefile` |
| One-command installer (curl\|sh bridge: clone + bootstrap + install) | `install.sh` |

Your editable config is `config/`; the launcher code is `src/`. The container
definition (`docker/`, `.devcontainer/`, `docker-compose.yml`, `.claude-plugin/`)
stays at the repo root where Docker and the devcontainer CLI expect it — tool
code, not config.

## Architecture in one breath

A `.devcontainer` driven by `@devcontainers/cli`. The image is built from
`docker/Dockerfile`; the Claude Code CLI and `gh` are added as official **Features**
(consumed, not vendored). Egress has two layers: all container traffic is routed
through a host-side proxy so it rides your Mac's network, and the agent's Bash is
separately confined by Claude Code's native sandbox allowlist — no iptables, no
`NET_ADMIN`. Per-start setup runs in the container **entrypoint** (idempotent), so
every start path — `mirabilis`, `docker compose up`, or Docker auto-restart — comes
up configured. Persistent volumes (`~/.claude`, `~/.config/gh`) keep memory and auth
across updates; `/workspace` is a named volume the sandbox owns (opened via VSCode attach); everything else is ephemeral.
At launch the neuro-matrix protocol is appended to the system prompt and its hooks
add the invariant and verification gates. mirabilis does not protect files inside
the container — the container is the boundary (I5) and inside the agent has full
freedom (I2); behavioural limits are the harness's job, not the sandbox's. The host
preflight fails fast if egress or Claude access is missing; the harness is optional
(a warning, not a stop — you can opt out and run bare).
