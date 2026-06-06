# mirabilis

> An autonomous Claude Code workspace on macOS: one command gives you an isolated
> dev container with the
> [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) harness, persistent
> memory, and a configurable egress allowlist.

## Requirements

macOS (Apple Silicon or Intel) with [Homebrew](https://brew.sh). Docker Desktop and
the devcontainer CLI are installed for you on first run.

## Quick start

```sh
git clone https://github.com/AlexShchuka/mirabilis.git && mirabilis/mirabilis
```

That one line is the whole setup. The first run installs prerequisites, puts
`mirabilis` on your PATH, builds the container, and signs you in to GitHub and
Claude (native flows, saved in the sandbox). After that, run it from anywhere:

```sh
mirabilis
```

Put the repos you want it to work on in `~/mirabilis-workspace` (mounted at
`/workspace`). An IDE's *Reopen in Container* attaches to the same workspace.

## Updating

```sh
mirabilis update
```

Pulls the latest version and rebuilds; your memory, auth, and `/workspace` are kept.
`mirabilis` also warns you on launch when your checkout or container is behind.

## More

- `mirabilis help` — every command. Docker Desktop must be running; `mirabilis`
  tries to start it for you.
- [`AGENTS.md`](AGENTS.md) — architecture and contributor rules.
- [`SECURITY.md`](SECURITY.md) — threat model and secret handling.

## License

[MIT](LICENSE) © 2026 AlexShchuka
