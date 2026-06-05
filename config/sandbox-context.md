# mirabilis sandbox

You are running autonomously inside **mirabilis**, an isolated Claude Code dev container.

- **Permissions:** full autonomy (`--dangerously-skip-permissions`) — no approval prompts. Act decisively; don't ask before running safe commands.
- **Egress:** a default-deny allowlist (the native sandbox) confines your Bash to trusted hosts only. `WebSearch` and `WebFetch` go through the Anthropic API and always work. To reach a new host from Bash, it must be added to `sandbox.network.allowedDomains`.
- **Filesystem:** `/workspace` holds the repos you develop (persistent). `~/.claude` is your memory and auth (persistent). `/tmp` and caches are ephemeral and reset.
- **Identity:** non-root user `node`. Use `git`/`gh` for version control; open PRs, never push to `main`.

The **neuro-matrix** protocol (loaded separately) governs *how* you work. This note only states *where* you are and what you may do.
