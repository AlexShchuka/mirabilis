# mirabilis

> An autonomous Claude Code workspace on macOS. Clone it, run one command, and
> you get an isolated dev container with the
> [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) plugin
> preinstalled, persistent memory, and a configurable egress allowlist.

## Requirements

- macOS on Apple Silicon or Intel, with [Homebrew](https://brew.sh) installed.
  Docker Desktop and the devcontainer CLI are installed for you on first run.

## Quick start

```sh
git clone https://github.com/AlexShchuka/mirabilis.git
cd mirabilis
./mirabilis
```

The first run does everything: installs any missing prerequisites, puts the
`mirabilis` command on your PATH, builds the container, sets up your GitHub token
and Claude login, and opens the agent. From then on, from anywhere:

```sh
mirabilis
```

Put the repos you want it to work on in `~/mirabilis-workspace` (mounted at
`/workspace`). The runtime is a standard `.devcontainer`, so an IDE's *Reopen in
Container* attaches to the same workspace.

## Updating

```sh
mirabilis update
```

Pulls the latest version and rebuilds the image and container from scratch — your
memory, auth, and `/workspace` are kept. `mirabilis` also **warns you on launch**
when your checkout is behind the remote, or your running container is behind your
checkout; it still starts, so you can update when it suits you.

## What's inside

- Built on Anthropic's official dev-container **Feature** for the Claude Code CLI
  (consumed, not vendored) — with a thin mirabilis layer on top.
- **neuro-matrix** plugin, **GitHub** + **Context7** MCP, and a native status line — preinstalled.
- Egress is a **configurable allowlist** via Claude Code's native sandbox
  (`sandbox.network.allowedDomains`) — no iptables, no elevated capabilities. To
  let the agent's shell reach a new host, add it to that list in
  `config/settings.json`.
- Secrets live in the **macOS Keychain**, never in the repo. The agent runs **non-root**.

## Troubleshooting

- Run `mirabilis doctor` for a health check (Docker, secrets, sandbox, plugin,
  MCP, version, and egress).
- Docker Desktop must be running; `mirabilis` tries to start it, but launch it
  yourself if it does not come up.
- The first-run Claude login is geo-restricted — connect your VPN to a supported
  region before it runs.
- `mirabilis help` lists every command.

## Power users / CI

The `make` targets expose the lifecycle directly: `make bootstrap`
(prerequisites), `make install` / `make uninstall` (the PATH launcher),
`make doctor`, and `make clean` (remove the container and image; volumes are
kept). `make install PREFIX=…` overrides where the launcher is written (the
default is your Homebrew prefix's `bin`, which is already on PATH).

## License

[MIT](LICENSE) © 2026 AlexShchuka
