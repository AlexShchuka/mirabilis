# AGENTS.md

**mirabilis** is a personal, open-source macOS dev container that runs Claude Code with
full autonomy, the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) harness
preinstalled, persistent memory, and open egress. `CLAUDE.md` is a
symlink to this file. Setup: `README.md`. Threat model: `SECURITY.md`.

## Non-goals

Not a multi-user platform, not a hosted service, not reproducible (use-latest, no version
pinning except GitHub Actions refs), not a security gate against the agent itself. KISS
beats reliability beats security-from-exfiltration.

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

`config/` editable config (settings seed, plugins, stacks, apt-packages, memory rules) ·
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
  JSON, YAML, `.env`).
- Never commit secrets. Sign-in is native and persists in volumes; the only host-side
  secret is the optional Telegram token, read from the macOS Keychain in `internal/runtime`.
  If a token appears in a diff, stop.
- Don't push to `main`; branch and open a PR. Keep the PR description minimal — what and
  why in a few lines, no ceremony.
- Don't restate a behaviour here — change it in its owning file.

## Development principles

Operating agreement for working **on this repo** (complements the harness's general HOW):

1. **Code is truth.** Back every claim about state with tool output — a concrete run with concrete output, no plausible-sounding excuses.
2. **Minimal diff / YAGNI.** Touch only what's broken or what the task needs; don't refactor unrelated code or add layers "just in case".
3. **Anti-neuroslop.** Plausible shape ≠ needed code; don't grow files/abstractions detached from what the repo needs (the removed `tinyproxy` — "a pipe, not a filter" — was the example).
4. **Discuss → plan → code only on an explicit go.** Don't write code on unsettled requirements.
5. **Not green means not done.** Cover changes with tests.
6. **No churn.** One coherent landing, not a sequence of rewrites.

The mechanical toolchain (`go test`, bats — see `.github/workflows/ci.yml`) is the source of
mechanical rules; don't duplicate them in prose. Go code follows the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [Effective Go](https://go.dev/doc/effective_go), and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments); code is `gofmt`/`goimports`-clean (CI enforces `gofmt`). **How** to work in general lives in the
neuro-matrix harness; this file says **what** the repo is, plus the repo-specific principles
above.
