# mirabilis

> *Annus Mirabilis, 1905.* A local, reproducible, isolated **workspace** on macOS
> where an autonomous Claude Code agent develops your repositories — non-root,
> bypass-permissions, behind an egress allowlist, with the
> [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) plugin preinstalled.

mirabilis is an independent repo that packages the sandbox engine. It follows the
Coder workspace model (persistent vs ephemeral filesystem), runs the agent inside
a Docker container as the security boundary, and integrates the `neuro-matrix`
plugin natively through its own Claude Code marketplace.

See [`docs/adr/`](docs/adr/) for the decisions and [`AGENTS.md`](AGENTS.md) for the
rules any agent (including Claude) follows when working in this repo.

## Architecture

- **Engine** — Docker Desktop + Compose. The container is the boundary.
- **Filesystem** — persistent (`~/.claude` auth/memory/plugins, `~/.config/gh`,
  `/workspace`) survives restarts; ephemeral (`/tmp`, caches) is fresh each start.
- **Network** — geo-exit is your **host VPN**; the in-container firewall is a
  default-deny allowlist (`docker/init-firewall.sh`), defence-in-depth.
- **Auth** — the Claude OAuth token and GitHub token live in the **macOS Keychain**
  and are injected as environment variables at `make up`; never written to the repo.
- **Agent** — `claude --dangerously-skip-permissions` as the non-root `coder` user.

## Prerequisites

- macOS (Apple Silicon) with [Homebrew](https://brew.sh).
- A **VPN to a Claude-supported country** turned on at the host level. Claude is
  geo-restricted; nothing reaches the API until host traffic exits a supported
  region (`make doctor` verifies this).
- A Claude **Pro/Max** subscription (for `setup-token`) and a **GitHub token**.

## Quickstart (macOS)

```sh
git clone https://github.com/AlexShchuka/mirabilis.git
cd mirabilis

# before anything below: turn on your VPN (Claude-supported region) and start Docker Desktop
make bootstrap            # installs Docker Desktop via Homebrew — then open it
cp .env.example .env      # optional: set WORKSPACE_DIR (defaults to ~/mirabilis-workspace)

make token-gh             # paste your GitHub token (hidden, stored in the Keychain)
make token-context7       # optional: paste a Context7 API key (ctx7sk_…)

make build                # builds the image with a pinned Claude Code CLI
make up                   # starts the workspace

make setup-token          # opens the Claude OAuth flow in-container; prints a token
make token-claude         # paste that token (hidden, stored in the Keychain)
make restart              # picks up the Claude token

git clone <your-repo> "$HOME/mirabilis-workspace"   # repos the agent will work on (mounted at /workspace)

make doctor               # checks docker / tokens / VPN-exit / claude / plugin / MCP
make claude               # launch the autonomous agent
```

Your three repos go in `WORKSPACE_DIR` (mounted at `/workspace`) — clone them
there and edit them in your IDE; the agent edits the same files in the container.

## Daily use

```sh
# VPN on, Docker Desktop running
make up
make claude                       # interactive autonomous session
make agent P="fix the flaky test" # one headless prompt
make shell                        # a shell in the workspace as coder
```

> **Note:** from **2026-06-15**, headless `claude -p` (used by `make agent`) on a
> personal subscription draws from a separate monthly Agent SDK credit pool. Plan
> long autonomous loops accordingly (see [ADR 0002](docs/adr/0002-implementation-deltas.md)).

## Secrets & tokens

The macOS Keychain is the single source of truth. `make token-gh`,
`make token-claude`, and `make token-context7` store secrets there (input is
hidden); `make up` reads them out and injects them as environment variables for
that run only. Nothing is written to the repo — `.env` and `secrets/` are
gitignored. If a token is ever exposed, rotate it (see [SECURITY.md](SECURITY.md)).

## What's preinstalled

- **neuro-matrix plugin** — via the bundled `mirabilis` marketplace
  (`.claude-plugin/marketplace.json`), enabled at user scope.
- **GitHub MCP** — hosted HTTP (`api.githubcopilot.com`), token from the env.
- **Context7 MCP** — hosted HTTP (`mcp.context7.com`) for up-to-date library docs
  while writing code; works keyless, better with a Context7 API key.
- **Status line** — native Claude Code `statusLine` showing model, directory,
  git branch, and context usage.

Add or remove MCP servers by editing `scripts/provision-mcp.sh`.

## Repo layout

```
docker/        Dockerfile, entrypoint, firewall
scripts/       token.sh (Keychain), compose.sh, doctor.sh, provision-mcp.sh
config/        seeded settings.json + statusline.sh
.claude-plugin/ marketplace.json (preinstalls neuro-matrix)
docs/adr/      architecture decisions
Makefile       the entire UX
```

## Roadmap

This is the **minimal first cut** — enough to install and use. Planned hardening
(CI with shellcheck/hadolint/gitleaks, pinned reviewed plugin refs, deeper
VPN-exit checks, project runtimes, observability, and more) is tracked in the
repo's GitHub issues.

## License

[MIT](LICENSE) © 2026 Sasha (AlexShchuka)
