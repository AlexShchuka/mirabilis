# mirabilis — a Claude Code sandbox for autonomous AI coding agents on macOS

**mirabilis** is a one-command **macOS dev container** that runs **Claude Code** as an
**autonomous AI coding agent** in an isolated Docker sandbox. You get full bypass autonomy
behind a container boundary, the
[`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) agent harness preinstalled,
persistent memory, and open egress — a personal, open-source workspace
where the agent can build, research, and learn without touching your Mac.

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

macOS (Apple Silicon or Intel) with [Homebrew](https://brew.sh). Docker Desktop and the
devcontainer CLI are installed for you on first run.

## Quick start

```sh
curl -fsSL https://raw.githubusercontent.com/AlexShchuka/mirabilis/main/install.sh | bash
```

That one line clones mirabilis to `~/.mirabilis`, installs prerequisites, and puts
`mirabilis` on your PATH. The first launch builds the container and signs you in to GitHub
and Claude (native flows, saved in the sandbox). After that, run it from anywhere:

```sh
mirabilis
```

## Documentation

- [`AGENTS.md`](AGENTS.md) — what the repo is, boundaries, and contributor rules.
- [`SECURITY.md`](SECURITY.md) — threat model and secret handling.

## License

[MIT](LICENSE)
