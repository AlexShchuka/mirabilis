# AGENTS.md

**mirabilis** is a personal, open-source cross-platform dev container that runs Claude Code with
full autonomy, the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) harness
preinstalled, persistent memory, and open egress. `CLAUDE.md` is a
symlink to this file. Setup: `README.md`. Threat model: `SECURITY.md`.

## Boundaries

- The **container is the security boundary**. Inside it the agent has full freedom —
  root, `sudo`, any file. Behavioural limits (no push/force to `main`, no credential
  exfiltration) are the **harness's** job, not the sandbox's.
- Work lives in **`/workspace`**, a named volume the sandbox owns (opened via VS Code Dev
  Containers attach, not a host folder). `~/.claude` and `~/.config/gh` are persistent
  volumes (memory, auth). `/tmp` and everything else is ephemeral.
- Egress is **open**: the container reaches the network directly — no in-container allowlist,
  no proxy. Stopping credential exfiltration is the **harness's** behavioural job, not a
  network gate. `WebFetch`/`WebSearch` go via the Anthropic API and always work.

## Layout

`config/` editable config (settings seed, plugins, stacks, memory rules) ·
`cmd/mirabilis/` the single binary entry point — role dispatch (TUI / provision / hook) ·
`internal/` Go packages: TUI host launcher (`internal/app`), pipeline engine
(`internal/pipeline`), provisioner (`internal/provision`), OS seam and Docker primitives
(`internal/runtime`), Claude hook handlers (`internal/hooks`), step implementations
(`internal/steps/*`), GitHub device-flow (`internal/ghauth`), and supporting leaves
(`internal/runner`, `internal/ui`, `internal/config`) ·
`.devcontainer/` + `docker-compose.yml` the container definition (Dockerfile for build-time
layers; `devcontainer.json` for features and lifecycle hooks) · `test/` the install.sh
bats smoke.

## Don't

- No comments in code or config — prose lives in `.md` only (shell, Dockerfile, Makefile,
  JSON, YAML, `.env`). One exception: the version comment after a SHA-pinned `uses:` in
  `.github/workflows` — tool-consumed metadata, not prose.
- Never commit secrets. There are two host-side secrets: the Claude OAuth token
  (`claude setup-token` on the host, stored in the macOS Keychain or
  `~/.claude/.mirabilis-claude-token` on Linux/WSL, injected as
  `CLAUDE_CODE_OAUTH_TOKEN`) and the optional Telegram token (macOS Keychain or
  `~/.claude/.mirabilis-telegram-token` on Linux/WSL, injected as
  `TELEGRAM_BOT_TOKEN`). Both are blocked from host-env pass-through by
  `blockedFromContainer` (`internal/runtime`). If a token appears in a diff, stop.
- Don't push to `main`; branch and open a PR. Keep the PR description minimal — what and
  why in a few lines, no ceremony.
- Every PR merge to `main` auto-releases a patch bump; label `release:minor` / `release:major` to raise; direct pushes don't release.

## Development principles

Operating agreement for working **on this repo** (complements the harness's general HOW):

1. **Code is truth.** Back every claim about state with tool output — a concrete run with concrete output, no plausible-sounding excuses.
2. **Minimal diff / YAGNI.** Touch only what's broken or what the task needs; don't refactor unrelated code or add layers "just in case".
3. **Anti-neuroslop.** Plausible shape ≠ needed code; don't grow files/abstractions detached from what the repo needs (the removed `tinyproxy` — "a pipe, not a filter" — was the example).
4. **Discuss → plan → code only on an explicit go.** Don't write code on unsettled requirements.
5. **Not green means not done.** Cover changes with tests.
6. **No churn.** One coherent landing, not a sequence of rewrites.
7. **Human-readable without AI.** An AI-generated artifact enters shared state only if a human can read and verify it without an AI.
8. **Human gate before protocol-level shared state.** Protocol artifacts (invariant table, glossary) need the non-producing human's sign-off, paired to Common Code v3 `signed_by` (neuro-matrix#29).
9. **BLUF.** Lead with the conclusion.

The mechanical toolchain (`go test`, bats — see `.github/workflows/ci.yml`) is the source of
mechanical rules; don't duplicate them in prose. Go code follows the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [Effective Go](https://go.dev/doc/effective_go), and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments); code is `gofmt`/`goimports`-clean (CI enforces `gofmt`). **How** to work in general lives in the
neuro-matrix harness; this file says **what** the repo is, plus the repo-specific principles
above.
