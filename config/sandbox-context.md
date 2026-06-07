# mirabilis sandbox

You are running autonomously inside **mirabilis**, an isolated Claude Code dev container.

- **Permissions:** full autonomy (`--dangerously-skip-permissions`) — no approval prompts. The container itself is the safety boundary; inside it you have full freedom, so act decisively. If you break your own container, a rebuild restores it.
- **Egress:** a default-deny allowlist (the native sandbox) confines your Bash to trusted hosts only. `WebSearch` and `WebFetch` go through the Anthropic API and always work. To reach a new host from Bash, it must be added to `sandbox.network.allowedDomains`.
- **Storage — know where things live:**
  - `/workspace` — the only place to clone repos and write code; shared with the human's editor. Work here; organize it however you like. Memory stays global (`~/.claude`) plus Claude's automatic per-project context.
  - `~/.claude` (your **persistent memory** + Claude credentials) and `~/.config/gh` (GitHub credentials) live in volumes inside the sandbox and survive updates and rebuilds. Keep them tidy — store durable memory here, not scratch.
  - `/tmp` and everything else is **ephemeral**: wiped on restart/update. Use it for scratch only; never keep anything you need there.
- **Identity:** non-root user `node` with passwordless `sudo`, already signed in to `git`/`gh`. Use `git`/`gh` for version control; open PRs, never push to `main`. Behavioural limits (no push to `main`, no credential exfiltration, etc.) come from the neuro-matrix harness below, not from any sandbox-side gate.

- **Harness:** the **neuro-matrix** protocol below is your core operating harness — injected as the base layer of this sandbox, not an optional add-on. When installed, the harness is rooted at `~/.neuro-matrix` (a stable symlink to it); the protocol's relative self-references (`references/…`, `invariants.txt`, `scripts/…`, `agents/…`) resolve there (e.g. `~/.neuro-matrix/invariants.txt`). If `~/.neuro-matrix` is absent the harness failed to install — surface that, don't guess.

The **neuro-matrix** protocol below governs *how* you work; this note states *where* you are. They are one coherent system.
