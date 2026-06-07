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
   the macOS Keychain (`scripts/token.sh`). If a token ever appears in a diff, stop.
3. **Branch, never push to `main`.** Work lands via one squash-merged PR per task;
   the source branch is deleted after merge.
4. **Work inside `/workspace`.** The single source of truth, shared with the host
   editor. Prefer your own branch or a git worktree over editing a file in use.
5. **Egress is a configurable allowlist.** New outbound hosts go in
   `config/settings.json` → `sandbox.network.allowedDomains`, deliberately.

## Commands

```
mirabilis            the only command — opens a menu: launch / update / sign-in / theme
```

Every run opens the menu, highlighting anything stale (the workspace container, the
mirabilis repo, the neuro-matrix harness); pick **launch** to drop into Claude. Update
and sign-in live in the menu, not as subcommands. `mirabilis completion zsh` and
`mirabilis version` are the only other invocations. The launcher is `bin/mirabilis` —
read it before changing any lifecycle behaviour.

## Where things live

Change a behaviour in its owning file, not by restating it here.

| Concern | File |
|---|---|
| Launcher, menu, staleness, preflight + fail-fast gate, egress proxy | `bin/mirabilis` |
| Image: base, system tools, Python + .NET SDK, RTK binary (pinned) | `docker/Dockerfile` |
| Volumes, `/workspace` mount, container env, proxy wiring, entrypoint | `docker-compose.yml` |
| Dev-container engine; Claude Code CLI + `gh` as official Features | `.devcontainer/devcontainer.json` |
| Container entrypoint: runs per-start setup on every start (incl. auto-restart) | `.devcontainer/entrypoint.sh` |
| Per-start setup (idempotent): settings seed, denyWrite render, theme, trust, plugins, MCP, apt-list, hook wiring | `.devcontainer/refresh.sh` |
| Consent gate (honest gate + deny-set; PreToolUse hook) | `scripts/consent-gate.sh` |
| Protected-paths single source (feeds the gate + sandbox denyWrite) | `config/protected-paths` |
| Declared apt packages (re-applied at start) | `config/apt-packages.txt` |
| Claude settings: sandbox allowlist + filesystem, plugins, theme | `config/settings.json` |
| Agent-facing sandbox note (prepended to the system prompt) | `config/sandbox-context.md` |
| Plugin marketplace (installs neuro-matrix, pinned `ref`) | `.claude-plugin/marketplace.json` |
| MCP registration (Context7) | `scripts/provision-mcp.sh` |
| Secret storage (macOS Keychain ↔ env) | `scripts/token.sh` |
| Prerequisites / install / clean (power users, CI) | `Makefile` |

## Architecture in one breath

A `.devcontainer` driven by `@devcontainers/cli`. The image is built from
`docker/Dockerfile`; the Claude Code CLI and `gh` are added as official **Features**
(consumed, not vendored). Egress has two layers: all container traffic is routed
through a host-side proxy so it rides your Mac's network, and the agent's Bash is
separately confined by Claude Code's native sandbox allowlist — no iptables, no
`NET_ADMIN`. Per-start setup runs in the container **entrypoint** (idempotent), so
every start path — `mirabilis`, `docker compose up`, or Docker auto-restart — comes
up configured. Persistent volumes (`~/.claude`, `~/.config/gh`) keep memory and auth
across updates; `/workspace` is the host bind-mount; everything else is ephemeral.
At launch the neuro-matrix protocol is appended to the system prompt and its hooks
add the invariant and verification gates; a separate consent-gate hook (with a
deny-set) plus the host preflight enforce sensitive-action consent and fail-fast.
