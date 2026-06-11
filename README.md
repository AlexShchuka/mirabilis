# mirabilis — a Claude Code sandbox for autonomous AI coding agents

[![ci](https://github.com/AlexShchuka/mirabilis/actions/workflows/ci.yml/badge.svg)](https://github.com/AlexShchuka/mirabilis/actions/workflows/ci.yml)

**mirabilis** is a one-command **cross-platform dev container** that runs **Claude Code** as an **autonomous AI coding agent** in an isolated
Docker sandbox. You get full bypass autonomy behind a container boundary, the
[`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) agent harness preinstalled,
persistent memory, and open egress — a personal, open-source workspace
where the agent can build, research, and learn without touching your PC.

## The name

*mirabilis* honors Einstein's *annus mirabilis* — his 1905 "miracle year", when four
landmark papers (special relativity, the photoelectric effect, Brownian motion, and
mass–energy equivalence, E=mc²) reshaped physics in a single year.

## Why mirabilis

- **Isolated by design** — the Docker container is the security boundary; inside, the agent
  has full freedom (root, `sudo`, any file), so it never needs approval prompts.
- **Autonomous Claude Code** — launches in bypass mode.
- **Open egress** — the container reaches the network directly; no host proxy, no in-container
  allowlist. Stopping credential exfiltration is the harness's job; `WebFetch`/`WebSearch` always work.
- **Persistent memory** — `~/.claude` live in volumes that survive rebuilds; path-scoped memory rules load per file type.
- **One TUI menu** — launch, plugins, harness, stack, open in VS Code — all from a single Go
  terminal UI. Each step retried under policy, then drops you into Claude.

## Requirements

- **macOS** — Homebrew; `install.sh` runs `make bootstrap` to install Docker Desktop, the
  devcontainer CLI, and Go.
- **Linux** — git, make, Go, Node + npm, and Docker Engine with the Compose v2 plugin.
  `install.sh` checks for these and prints exact install hints for whatever is missing; it
  never installs system packages for you.
- **Windows** — run everything inside **WSL2**: install WSL2
  (`wsl --install -d Ubuntu-24.04`), then follow the Linux steps inside the distro. Either
  enable Docker Desktop's WSL integration for the distro, or install docker-ce inside WSL.

## Quick start

```sh
curl -fsSL https://raw.githubusercontent.com/AlexShchuka/mirabilis/main/install.sh | bash
```

That one line clones mirabilis to `~/.mirabilis`, installs the devcontainer CLI, and puts
`mirabilis` on your PATH. The first launch builds the container and signs you in to
GitHub (native flow, saved in the sandbox). For Claude, run `claude setup-token` on the
host once — the resulting OAuth token is stored in your host keychain (macOS) or a
host-side secret file (Linux/WSL) and injected at container launch. After that, run it
from anywhere:

```sh
mirabilis
```

## Documentation

- [`AGENTS.md`](AGENTS.md) — what the repo is, boundaries, and contributor rules.
- [`SECURITY.md`](SECURITY.md) — threat model and secret handling.

## License

[MIT](LICENSE)
