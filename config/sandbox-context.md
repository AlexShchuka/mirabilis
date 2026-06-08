# mirabilis sandbox

You are running autonomously inside **mirabilis**, an isolated Claude Code dev container.
This note says **where** you are; the neuro-matrix protocol below says **how** to work.

- **Permissions:** full autonomy (`--dangerously-skip-permissions`) — no approval prompts.
  The container is the safety boundary; inside it you have full freedom, so act decisively.
  Break your own container and a rebuild restores it.
- **Egress:** a default-deny allowlist (the native sandbox) confines your Bash to trusted
  hosts only. `WebSearch` and `WebFetch` go through the Anthropic API and always work. To
  reach a new host from Bash, add it to `sandbox.network.allowedDomains`.
- **Storage — know where things live:**
  - `/workspace` — the only place to clone repos and write code; a named volume the sandbox
    owns (not a host folder), opened from your editor via VS Code (Dev Containers attach).
    Work here; organize it however you like.
  - `~/.claude` (your **persistent memory** + Claude credentials) and `~/.config/gh`
    (GitHub credentials) live in volumes inside the sandbox and survive updates and
    rebuilds. Keep them tidy — store durable memory here, not scratch.
  - `/tmp` and everything else is **ephemeral**: wiped on restart/update. Scratch only; no
    inbox, no drop folder — bring files in by editing `/workspace` over the VS Code attach.
- **Memory layout:**
  - Auto-memory is native and enabled; durable facts persist in `~/.claude`.
  - `~/.claude/rules/*.md` carry **path-scoped, per-file-type** rules via YAML `paths:`
    glob frontmatter (e.g. `python.md` → `**/*.py`), so the right rules load for whatever
    work runs in the one container.
  - Tag durable memory by domain so it is greppable: `#dev` · `#science` · `#interview` ·
    `#study`.
- **Identity:** non-root user `node` with passwordless `sudo`, already signed in to
  `git`/`gh`. Use `git`/`gh` for version control; open PRs, never push to `main`.

The **neuro-matrix** protocol below governs *how* you work, including behavioural limits
(not breaking others' work, not exfiltrating credentials) — those come from the harness,
not from any sandbox-side gate. This note and the protocol are one coherent system.
