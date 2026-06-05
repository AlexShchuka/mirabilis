# mirabilis

> An autonomous Claude Code workspace on macOS. One command starts an isolated
> sandbox with the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix)
> plugin preinstalled, persistent memory, and a default-deny egress firewall.

## Install (once)

```sh
git clone https://github.com/AlexShchuka/mirabilis.git
cd mirabilis
make bootstrap     # Docker Desktop (via Homebrew)
make install       # puts `mirabilis` on PATH
```

## Use

Turn on your VPN (Claude is geo-restricted), then:

```sh
mirabilis
```

That is the whole daily workflow. The **first** run sets up your GitHub token and
Claude login automatically; every run after just opens the agent. Put the repos
you want it to work on in `~/mirabilis-workspace` (mounted at `/workspace`).

Other commands exist but you rarely need them — see `mirabilis help`.

## What's inside

- **neuro-matrix** plugin, **GitHub** + **Context7** MCP, and a native status line — preinstalled.
- Secrets live in the **macOS Keychain**, never in the repo.
- The agent runs **non-root** behind a **default-deny** egress allowlist; the
  geo-exit is your host VPN.
- Persistent (`~/.claude`, `/workspace`) vs ephemeral (`/tmp`, caches) filesystem.

## License

[MIT](LICENSE) © 2026 AlexShchuka
