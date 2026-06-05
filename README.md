# mirabilis

> An autonomous Claude Code workspace on macOS. One command starts an isolated
> dev container with the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix)
> plugin preinstalled, persistent memory, and a configurable egress allowlist.

## Requirements

- macOS on Apple Silicon or Intel, with [Homebrew](https://brew.sh) installed.
  Installing Homebrew also installs the Xcode Command Line Tools that the
  bootstrap step relies on.
- Everything else — Docker Desktop and the devcontainer CLI — is installed by
  `make bootstrap`.

## Install (once)

Clone the repository and install the prerequisites:

```sh
git clone https://github.com/AlexShchuka/mirabilis.git
cd mirabilis
make bootstrap
```

Then put the `mirabilis` command on your PATH:

```sh
make install
```

`make install` writes the launcher into your Homebrew prefix's `bin`
(`brew --prefix`, e.g. `/opt/homebrew/bin` on Apple Silicon), which is already on
your PATH — no `sudo`, no shell-config edits. To install elsewhere, override the
prefix and make sure its `bin` is on your PATH:

```sh
make install PREFIX="$HOME/.local"
```

## Use

```sh
mirabilis
```

That is the whole daily workflow. The **first** run sets up your GitHub token and
Claude login automatically; every run after just opens the agent. Put the repos
you want it to work on in `~/mirabilis-workspace` (mounted at `/workspace`).

Other commands — `mirabilis help`. The runtime is a standard `.devcontainer`, so
an IDE's *Reopen in Container* attaches to the same workspace.

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
  MCP, and egress).
- Docker Desktop must be running; `mirabilis` tries to start it, but launch it
  yourself if it does not come up.
- The first-run Claude login is geo-restricted — connect your VPN to a supported
  region before it runs.

## Uninstall

```sh
make uninstall
```

This removes the `mirabilis` command from your PATH. `make clean` additionally
removes the container and image (the persistent volumes are kept).

## License

[MIT](LICENSE) © 2026 AlexShchuka
