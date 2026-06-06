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
git clone https://github.com/AlexShchuka/mirabilis.git && mirabilis/mirabilis
```

That one line is the whole setup — no `cd`, nothing else to copy. The first run
installs any missing prerequisites, **puts `mirabilis` on your PATH globally**,
builds the container, and signs you in to GitHub and Claude (native flows, saved
in the sandbox). From then on, run it from anywhere in the terminal:

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

Runs `git pull` and rebuilds the image and container from scratch — your memory,
auth, and `/workspace` are kept. `mirabilis` also **warns you on launch**
when your checkout is behind the remote, or your running container is behind your
checkout; it still starts, so you can update when it suits you.

## What's inside

- Built on Anthropic's official dev-container **Feature** for the Claude Code CLI
  (consumed, not vendored) — with a thin mirabilis layer on top.
- Preinstalled: the **neuro-matrix** harness, the official **GitHub** plugin
  (bundles the GitHub MCP), **Context7** MCP, **RTK** (a token-saver that
  compresses command output 60–90% via a transparent Bash hook), and a native
  status line.
- **All container egress is routed through your Mac.** On launch `mirabilis`
  starts a small forward proxy as a host process and points the container at it
  (`HTTPS_PROXY` → `host.docker.internal`), so the workspace reaches the internet
  over the host's network — the same VPN or connection as your terminal — instead
  of Docker's own path. Launch verifies the container's exit IP matches the host's.
- Egress is also a **configurable allowlist** via Claude Code's native sandbox
  (`sandbox.network.allowedDomains`) — no iptables, no elevated capabilities. To
  let the agent's shell reach a new host, add it to that list in
  `config/settings.json`.
- Sign-in is native and saved **inside the sandbox** (`gh auth login` + Claude's first-run login → persistent volumes), never in the repo. The agent runs **non-root**. The `/workspace` folder is trusted automatically — no repeated "trust this folder" prompt.

## Troubleshooting

- Every `mirabilis` launch runs a built-in preflight (Docker, sandbox, plugin,
  MCP, and that container egress exits through your host) and prints any warning;
  it still starts so you can act when it suits you.
- Docker Desktop must be running; `mirabilis` tries to start it, but launch it
  yourself if it does not come up.
- On first run you sign in to GitHub and Claude — a code and URL appear; open the
  URL in your browser, approve, and paste the code back. It is saved in the
  sandbox volumes, so you are not asked again.
- `mirabilis help` lists every command.

## Power users / CI

The `make` targets expose the lifecycle directly: `make bootstrap`
(prerequisites), `make install` / `make uninstall` (the PATH launcher),
and `make clean` (remove the container and image; volumes are
kept). `make install PREFIX=…` overrides where the launcher is written (the
default is your Homebrew prefix's `bin`, which is already on PATH).

## License

[MIT](LICENSE) © 2026 AlexShchuka
