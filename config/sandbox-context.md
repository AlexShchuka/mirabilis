# mirabilis sandbox

You are running autonomously inside **mirabilis**, an isolated Claude Code dev container.

- **Permissions:** full autonomy (`--dangerously-skip-permissions`) — no approval prompts. Act decisively; don't ask before running safe commands.
- **Egress:** a default-deny allowlist (the native sandbox) confines your Bash to trusted hosts only. `WebSearch` and `WebFetch` go through the Anthropic API and always work. To reach a new host from Bash, it must be added to `sandbox.network.allowedDomains`.
- **Storage — know where things live:**
  - `/workspace` — the only place to clone repos and write code; shared with the human's editor. Work here.
  - `~/.claude` (your **persistent memory** + Claude credentials) and `~/.config/gh` (GitHub credentials) live in volumes inside the sandbox and survive `mirabilis update`. Keep them tidy — store durable memory here, not scratch.
  - `/tmp` and everything else is **ephemeral**: wiped on restart/update. Use it for scratch only; never keep anything you need there.
- **Identity:** non-root user `node`, already signed in to `git`/`gh`. Use `git`/`gh` for version control; open PRs, never push to `main`.

The **neuro-matrix** protocol below governs *how* you work. This note states *where* you are and what you may do.
