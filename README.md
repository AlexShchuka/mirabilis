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
- **Open egress** — the container reaches the network directly; `WebFetch`/`WebSearch` always
  work.
- **Persistent memory** — `~/.claude` lives in a volume that survives rebuilds; path-scoped
  memory rules load per file type.
- **One TUI menu** — launch, plugins, harness, stack, open in VS Code — all from a single Go
  terminal UI. Each step is idempotent: a repeat launch on a healthy system goes straight to
  Claude with zero questions and zero changes.
- **Secure auth chain** — the real Claude OAuth token never enters the container. The chain
  is: in-container claude → headroom (observability + MCP, port 8787) → host auth proxy
  (injects the real Bearer) → api.anthropic.com. The container holds only a per-session key
  that is useless without the live host process.

## Requirements

- **macOS** — Homebrew; `install.sh` runs `make bootstrap` to install Docker Desktop, the
  host claude CLI, and Go.
- **Linux** — git, make, Go, and Docker Engine with the Compose v2 plugin. `install.sh`
  checks for these and prints exact install hints for whatever is missing; it never installs
  system packages for you.
- **Windows** — run everything inside **WSL2**: install WSL2
  (`wsl --install -d Ubuntu-24.04`), then follow the Linux steps inside the distro. Either
  enable Docker Desktop's WSL integration for the distro, or install docker-ce inside WSL.

## Quick start

```sh
curl -fsSL https://raw.githubusercontent.com/AlexShchuka/mirabilis/main/install.sh | bash
```

That one line clones mirabilis to `~/.mirabilis`, installs the host claude CLI, and puts
`mirabilis` on your PATH. Run it from anywhere:

```sh
mirabilis
```

The first launch builds the container image (claude-code, gh, and docker-ce-cli baked in),
starts the container, provisions it, signs you in to Claude (via the menu's auth step) and
to GitHub (native device flow, saved in the sandbox), and drops you into Claude. Repeat
launches are instant — all steps are idempotent and skip when already satisfied.

## Documentation

- [`AGENTS.md`](AGENTS.md) — what the repo is, boundaries, and contributor rules.
- [`SECURITY.md`](SECURITY.md) — threat model and secret handling.

## License

[MIT](LICENSE)
