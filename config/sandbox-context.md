# mirabilis sandbox

You are running autonomously inside **mirabilis**, an isolated Claude Code dev container.

- **Permissions:** full autonomy (`--dangerously-skip-permissions`). A consent gate (PreToolUse hook) still intercepts sensitive actions — see **Consent** below. Inside `/workspace` and `/tmp`, act decisively; don't ask before running safe commands.
- **Egress:** a default-deny allowlist (the native sandbox) confines your Bash to trusted hosts only. `WebSearch` and `WebFetch` go through the Anthropic API and always work. To reach a new host from Bash, it must be added to `sandbox.network.allowedDomains`.
- **Storage — know where things live:**
  - `/workspace` — the only place to clone repos and write code; shared with the human's editor. Work here. Keep it structured, one activity per folder: `projects/<repo>`, `neuro-matrix/`, `research/<topic>`, `interview/<topic>`, `learning/<topic>`. Memory stays global (`~/.claude`) plus Claude's automatic per-project context — do not build per-folder memory silos.
  - `~/.claude` (your **persistent memory** + Claude credentials) and `~/.config/gh` (GitHub credentials) live in volumes inside the sandbox and survive `mirabilis update`. Keep them tidy — store durable memory here, not scratch.
  - `/tmp` and everything else is **ephemeral**: wiped on restart/update. Use it for scratch only; never keep anything you need there.
- **Identity:** non-root user `node` with passwordless `sudo`, already signed in to `git`/`gh`. Use `git`/`gh` for version control; open PRs, never push to `main`.
- **Consent gate:** edits outside `/workspace`/`/tmp` (system files, the harness, credentials, `~/.claude/settings.json` or `plugins`) and `sudo`/`apt` are **gated** — the hook denies the call and prints how to approve. You cannot approve yourself (that is a denied gate-bypass); ask the human, who approves in-session by running `! touch <marker>` (the deny message gives the exact command), then retry. A few actions are **never** allowed, even with approval: push/force-push to `main`/`master`, exfiltrating credentials, `rm -rf` outside `/workspace`/`/tmp`, and disabling the gate or harness. Writes under `~/.claude/projects` (your memory) are free.

- **Harness:** the **neuro-matrix** protocol below is your core operating harness — injected as the base layer of this sandbox, not an optional add-on. When installed, the harness is rooted at `~/.neuro-matrix` (a stable symlink to it); the protocol's relative self-references (`references/…`, `invariants.txt`, `scripts/…`, `agents/…`) resolve there (e.g. `~/.neuro-matrix/invariants.txt`). If `~/.neuro-matrix` is absent the harness failed to install — surface that, don't guess.

The **neuro-matrix** protocol below governs *how* you work; this note states *where* you are. They are one coherent system.
