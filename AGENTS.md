# AGENTS.md

**mirabilis** is a personal, open-source cross-platform dev container that runs Claude Code with
full autonomy, the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) harness
preinstalled, persistent memory, and open egress. `CLAUDE.md` is a
symlink to this file. Setup: `README.md`. Threat model: `SECURITY.md`.

## Boundaries

- The **container is the security boundary**. Inside it the agent has full freedom —
  root, `sudo`, any file. Behavioural limits (no push/force to `main`, no credential
  exfiltration) are the **harness's** job, not the sandbox's. Sandbox provisions;
  harness behaves (G2).
- Work lives in **`/workspace`**, a named volume the sandbox owns. `~/.claude` and
  `~/.config/gh` are persistent volumes (memory, auth). `/tmp` and everything else
  is ephemeral.
- Egress is **open**: the container reaches the network directly — no in-container allowlist.
  `api.anthropic.com` is reached via the chain: claude → headroom (`:8787`, observability +
  MCP) → host auth proxy (token injection) → Anthropic. The **real Claude token never enters
  the sandbox** — only a per-session key does. Stopping credential exfiltration is the
  **harness's** behavioural job. `WebFetch`/`WebSearch` go via the Anthropic API and always
  work.
- **Secrets**: single source of truth per platform — keychain (macOS) / file `0600`
  (Linux/WSL). Entry name `mirabilis-<key>` (no doubled suffix). The Claude OAuth token
  stays on the host; only the per-session key reaches the container. The Telegram token is
  host-side; Telegram chat-id is written to `.mirabilis/chat-id` (not an env var).

## Layout

```
cmd/mirabilis          dispatch: (no args→tui) | provision --phase | hook | notify send
│
├── internal/bus       message types + addressing (pure Go, no tea import)
├── internal/obs       slog sink + status registry: one log file, node-status aggregation
│
├── internal/engine    zero bubbletea dependencies
│   ├── exec           port Runner (Host/Container, line streaming) + adapters: dockersdk, fake
│   ├── secrets        port Store + adapters: keychain(darwin), file(linux/wsl)
│   ├── claudeauth     port TokenSource + adapter setuptoken (pty-interposer)
│   ├── authproxy      reverse proxy for api.anthropic.com (host goroutine, started at boot)
│   ├── sandbox        compose up/build/down/reset, inspect/events(SDK), fingerprint, attach
│   ├── pipeline       Command contract + FSM (deps, optional, retry, timeout, stream, Resume)
│   ├── steps          host launch-step implementations
│   ├── provision      container-side provision steps (create|start|plugins|skills)
│   ├── notify         port Notifier + adapter telegram (host-side; outbox watcher goroutine)
│   ├── membackup      port Save (host dockercp)
│   ├── status         container inspect snapshot + docker-events resubscribe loop
│   └── config         stacks/plugins/skills catalogs + persisted choices + tunables
│
└── internal/tui       rendering and dispatch only
    ├── app            root: owns frame+router, bridges engine events↔bus, runs tea.Exec
    ├── frame          persistent chrome: header(status), left menu panel, footer(hints)
    ├── router         screen stack in main area; addressed Envelope delivery
    ├── screens        menu, launch, telegram, harness, reset
    ├── components     steplist, cmdlog, form(huh wrapper), statusbar
    └── strings, styles  all UI strings (EN) and styles — one node each
```

`.devcontainer/` + `docker-compose.yml` the container definition (claude-code, gh, and
docker-ce-cli baked into the Dockerfile; docker.sock absent by default, enabled via
`compose.sock.yml` override) · `test/` the install.sh bats smoke.

## Don't

- No comments in code or config — prose lives in `.md` only (shell, Dockerfile, Makefile,
  JSON, YAML, `.env`). Two exceptions, both tool-consumed metadata not prose: the version
  comment after a SHA-pinned `uses:` in `.github/workflows`, and `# shellcheck disable=`
  directives in workflow `run:` scripts where the suppression is structurally correct.
- Never commit secrets. The Claude OAuth token is produced by `claude setup-token` on the
  host and stored in the macOS Keychain or `~/.claude/.mirabilis-claude` on Linux/WSL. It
  never enters the container (the chain uses a per-session key instead). The optional
  Telegram token lives in the Keychain or `~/.claude/.mirabilis-telegram` on Linux/WSL. If
  a token appears in a diff, stop.
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

The mechanical toolchain (`go test`, bats, golangci-lint — see `.github/workflows/ci.yml`) is
the source of mechanical rules; don't duplicate them in prose. Go code follows the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [Effective Go](https://go.dev/doc/effective_go), and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments); formatting and static analysis are CI-enforced. **How** to work in general lives in the
neuro-matrix harness; this file says **what** the repo is, plus the repo-specific principles
above.
