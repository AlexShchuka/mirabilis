# mirabilis

> An autonomous Claude Code workspace on macOS. One command starts an isolated
> dev container with the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix)
> plugin preinstalled, persistent memory, and a configurable egress allowlist.

## Install (once)

```sh
git clone https://github.com/AlexShchuka/mirabilis.git
cd mirabilis
make bootstrap     # Docker Desktop + the devcontainer CLI
make install       # puts `mirabilis` on PATH
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
  (`sandbox.network.allowedDomains`) — no iptables, no elevated capabilities.
- Secrets live in the **macOS Keychain**, never in the repo. The agent runs **non-root**.

## License

[MIT](LICENSE) © 2026 AlexShchuka
