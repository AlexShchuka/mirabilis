# AGENTS.md

Canonical agent guide for **mirabilis** — an autonomous Claude Code workspace.
Tool-neutral by design; `CLAUDE.md` is a symlink to this file.

## What this repo is

mirabilis is a Docker-based, reproducible, isolated workspace on macOS where an
autonomous Claude Code agent develops repositories. It runs the agent as a
non-root user with `--dangerously-skip-permissions`, behind an in-container
egress allowlist, and preinstalls the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix)
plugin through a self-owned Claude Code marketplace.

## Golden rules for any agent working here

1. **No comments in code or config.** Code describes itself. All prose belongs
   in Markdown docs. This applies to shell, Dockerfile, Makefile, YAML, `.env`,
   `.gitignore` — everything except `.md` files.
2. **Never commit secrets.** Tokens live in the macOS Keychain (`scripts/token.sh`)
   and are injected as environment variables at run time. `secrets/` and `.env`
   are gitignored. If you ever see a token in a diff, stop.
3. **Branch, don't push to `main`.** Open PRs with `gh`. `gh` will not push to
   `main` without confirmation.
4. **Work inside `/workspace`.** That bind-mount is the single source of truth,
   shared with the host IDE. Avoid editing a file the human is editing; prefer a
   git worktree or your own branch.
5. **Respect the egress allowlist.** New outbound hosts go in
   `docker/init-firewall.sh` (or `ALLOWLIST_EXTRA`), deliberately.

## Commands

```
make build        build the image (pinned Claude Code CLI)
make up           start the workspace
make doctor       diagnose docker / tokens / VPN-exit / claude / plugin / MCP
make claude       launch the autonomous agent
make agent P=...  one headless prompt
make shell        shell into the workspace as coder
```

## How it fits together

- **Engine:** Docker Desktop + Compose. The container is the security boundary.
- **Filesystem:** persistent (`~/.claude` auth+memory+plugins, `~/.config/gh`,
  `/workspace`) vs ephemeral (`/tmp`, caches). State that must live is persistent;
  everything else is fresh on restart.
- **Network:** geo-exit is the host VPN; the container firewall is a default-deny
  allowlist (defence-in-depth for the autonomous agent).
- **Plugin:** `.claude-plugin/marketplace.json` defines the `mirabilis`
  marketplace, which installs `neuro-matrix` at user scope on start. A plugin's
  root `CLAUDE.md` is **not** loaded as context — neuro-matrix injects its
  protocol via a SessionStart hook.
- **MCP:** GitHub MCP (hosted HTTP) is registered declaratively in
  `scripts/provision-mcp.sh`; the token is read from the environment.

See `docs/adr/` for the decisions and `README.md` for setup.
