# ADR 0002 — Implementation deltas vs ADR 0001

- **Status:** Accepted · **Date:** 2026-06-05 · **Supersedes parts of** [0001](0001-claude-sandbox.md)

These are corrections discovered by verifying ADR 0001's Claude Code assumptions
against current documentation before implementation. Where 0001 and reality
disagree, the implementation follows reality and this record explains why.

## D-1 — A plugin's root `CLAUDE.md` is NOT loaded as context

ADR 0001 (D4) treats `neuro-matrix` as a "system prompt" delivered by the
plugin's `CLAUDE.md`. The docs are explicit: *"A `CLAUDE.md` file at the plugin
root is not loaded as project context. Plugins contribute context through skills,
agents, and hooks."* **Implementation:** the plugin must inject its protocol via
a **SessionStart hook** (or a skill). mirabilis still preinstalls and enables the
plugin; making the context actually load is a neuro-matrix concern (tracked as an
issue).

## D-2 — Plugin integration is marketplace + `enabledPlugins`, not a bespoke clone

ADR 0001 cloned the plugin to an ephemeral dir and passed `--plugin-dir`.
`--plugin-dir` is real but single-session. The native, persistent path is a
**marketplace** (`.claude-plugin/marketplace.json`) plus `enabledPlugins` in
settings. mirabilis ships its own `mirabilis` marketplace and installs
`neuro-matrix@mirabilis` at **user scope** (trusted, no prompt) on start.

## D-3 — `bypassPermissions` only via CLI flag

There is no `bypassPermissions` value for `permissions.defaultMode` in
settings.json. Bypass is set **only** by `--dangerously-skip-permissions`
(equivalently `--permission-mode bypassPermissions`). Root is refused; we run
non-root, matching the Anthropic reference devcontainer.

## D-4 — GitHub MCP via hosted HTTP removes the Node/npx pain

ADR 0001 (D10/R6) worried about npx cold-starts and baking stdio MCP into the
image. The official GitHub MCP is available as a **hosted HTTP** server at
`https://api.githubcopilot.com/mcp/` with a `Authorization: Bearer <token>`
header. Using it means **no Node, no npx, no docker-in-docker** for day-1 MCP.
The image installs the Claude Code CLI via the **native installer** (pinned,
`DISABLE_AUTOUPDATER=1`); `INSTALL_NODE` is an opt-in build arg for later.

## D-5 — Auth in the container is env-token based; `~/.claude` persists state

On Linux (the container), credentials would live in `~/.claude/.credentials.json`,
but mirabilis authenticates with `CLAUDE_CODE_OAUTH_TOKEN` injected from the
Keychain each start, so the persistent `~/.claude` volume is for **memory,
settings, and installed plugins**, not login.

## D-6 — ToS / autonomy caveat (affects O1 / R8)

The Claude Code CLI on your own machine is sanctioned for scripted/automated use
on a personal subscription. **However, from 2026-06-15, `claude -p` and Agent SDK
usage on subscription plans draw from a separate monthly Agent SDK credit pool.**
This materially affects long autonomous `-p` loops and is tracked as an issue.
