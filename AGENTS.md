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
  work. **Deliberate additional egress edge**: `internal/engine/localllm` POSTs to
  `host.docker.internal:1234` (LM Studio default port) to offload prompts to a host-local
  model. This edge is intentional and hardcoded to `http://host.docker.internal:1234/v1`; it
  carries only prompt text, never the Claude token. If an egress allowlist is added in future,
  it must include `host.docker.internal:1234`.
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

Operating agreement for working **on this repo**:

1. **Code is truth.** Back every claim about state with tool output — a concrete run with concrete output, no plausible-sounding excuses.
2. **Minimal diff / YAGNI.** Touch only what's broken or what the task needs; don't refactor unrelated code or add layers "just in case".
3. **Anti-neuroslop.** Plausible shape ≠ needed code; don't grow files/abstractions detached from what the repo needs.
4. **Discuss → plan → code only on an explicit go.** Don't write code on unsettled requirements.
5. **Not green means not done.** Cover changes with tests; `go test -race`, golangci-lint, and bats must all pass.
6. **No churn.** One coherent landing, not a sequence of rewrites.
7. **Human-readable without AI.** An AI-generated artifact enters shared state only if a human can read and verify it without an AI.
8. **Human gate before protocol-level shared state.** Protocol artifacts (invariant table, glossary) need the non-producing human's sign-off.
9. **BLUF.** Lead with the conclusion.

## Meta-goals (owner directives — outrank node-level convenience)

- **G0 System over nodes.** Architecture soundness is the primary goal; every decision is judged by what it does to the whole graph first. Patching nodes and hoping architecture emerges is what produced the original mess.
- **G1 Dense, readable graph.** Coherent, contradiction-free; no invented abstractions, no dangling leaves, no code-for-code's-sake. Fewer, more precise nodes beat more code.
- **G2 Sandbox ≠ harness.** The container provisions and runs tools; mirabilis owns provisioning, autonomy, reproducibility, thread-safety, speed. Behavioural logic must not leak into sandbox nodes.
- **G3 SRP.** One responsibility per node; a bug localizes to one directory. Logic in `engine`, rendering in `tui`, never mixed.
- **G4 Config separate from logic.** All tunables live in `config/`; code only reads them. Behaviour changes without editing code.
- **G5 Single observability sink.** Every node's logs and status converge on one destination: the slog file + TUI status header, both fed by the same event stream. Nothing writes into the void.
- **G6 Graceful degradation.** A node's failure degrades its own function, not the system. Optional steps cascade-skip; every external call has a timeout; the menu keeps working when notify/proxy/status fails.
- **G7 Idempotency everywhere.** Every operation is safe to repeat: `Check→Run→Check` is the step contract; same input → same result; a repeat launch on a healthy system = zero questions, zero changes.
- **G8 Replaceable nodes.** Ports & adapters: swapping an adapter means one new file + one registration line, zero edits elsewhere.

## Invariants

Every invariant has a **mechanical home** — a linter rule, a contract test, or a CI check.
Prose here only identifies the home; the home is what actually enforces it.

| ID | Invariant | Mechanical enforcement |
|----|-----------|------------------------|
| I1 | Real Anthropic token never in the container (not env, not fs, not image, not `settings.json`) | e2e: `docker inspect` + exec env dump + grep token in container |
| I2 | UI thread does no I/O | **forbidigo** (`os/exec`, fs, `net/http`) in `tui/**` except `tui/app` Cmd ctors; latency golden |
| I3 | Every non-terminal step idempotent: Run ⇒ Check=true | pipeline contract test (§8.2) |
| I4 | Every command visible: spawn only via Runner; Runner always emits `started{Argv}` to cmdlog + obs log | **forbidigo** (raw `os/exec` banned); bus unit test |
| I5 | Host process leaves no children after exit | e2e: `ps` after quit |
| I6 | Port adapter swap = new file + registration; zero edits elsewhere | structure review; demo adapter test |
| I7 | A secret lives in exactly one backend per platform; old locations migrated then deleted | secrets unit + migration unit |
| I8 | All UI strings in `tui/strings`, English | grep test |
| I9 | Repeat launch on a healthy system: zero questions, zero changes | e2e checklist |
| I10 | docker.sock absent by default; enabling changes fingerprint | fingerprint unit + e2e inspect mounts |
| I11 | Every package has one responsibility; no dead code after switch | PR diff review |
| I12 | A single node's failure never blocks the menu; status shows `degraded` | unit (fault-injected adapter) + user-scenario test |
| I13 | One observability sink: every node logs via `obs`; nothing writes elsewhere | **forbidigo** (`^os\.Stderr$` — excluded for `cmd/mirabilis`, `internal/hooks`, `internal/obs`, `_test.go`; stray `log.*`); review |
| D2/G2 | `internal/engine/**` never imports `internal/tui`, `internal/bus`, or bubbletea/charmbracelet | **depguard** rule `engine-no-tui` |
| §4.2 | `tui/{screens,components,frame,router}` never import `internal/engine` | **depguard** rule `tui-leaves-no-engine` |
| D10 | No comments in code or non-workflow config | **errcheck** excludes are curated (no real errors hidden); CI `no-config-comments` job; pre-commit hook |
| BAR | No swallowed errors | **errcheck** (global excludes: best-effort Close in defers only; path-scoped: provision CLI stdout prints, httptest handler writes in tests; every other intentional ignore is an explicit `_ =` at the call site) |
| I1/secrets | gitleaks catches committed secrets | **gitleaks** CI job; pre-commit gitleaks hook (no-op if not installed) |

## Engineering bar

These are not style preferences — violating them is a defect.

**Fail-fast / error-as-data.** A process emits its outcome including failure; the global handler owns error handling and decides what to show. Never swallow an error or return false success — false success is a security defect (a step that silently claims completion when it failed leaves the system in an undefined state). Errors flow as values to the single obs sink.

**No-hang / no-race.** Every wait has a deadline and an escape path. Long-lived resources (the auth proxy, the docker-events watcher, the notify watcher) are owned for the session and stopped on context cancellation — they are never re-started mid-session, which avoids double-subscription races. Every goroutine launched must have a defined lifetime. `go test -race` is the gate.

**Code-is-truth.** "Not green = not done." No PR ships without all three gates passing: `go test -race ./...`, `golangci-lint run ./...`, `bats`. A claim about system behaviour must be backed by a passing test, not reasoning. The CI coverage floor (`floor=` in `ci.yml`) is a ratchet: raise it alongside real coverage gains, never lower it to make a PR pass; the current value of 86 was set in PR#126 with owner sign-off.

**TUI test determinism.** Bubble Tea v2 renders cell-level diffs, so substring waits on intermediate teatest frames are non-deterministic by construction (a string sharing a prefix or screen position with prior text may never appear contiguously, or may match a stale repaint). State-machine semantics are tested by driving `App.Update` synchronously in package-internal tests (`state_test.go`): pipeline events are consumed from `Events()` directly, assertions read model state (`pipe`, `busy`, `menuAction`, router depth, `Menu.Notice()`). teatest/pty harnesses are reserved for whole-program integration (exec handoff, golden frames, latency) and assert only final or probed state (`FinalOutput`, quit-probe), never intermediate frame substrings.

## References

Style and idioms are CI-enforced by golangci-lint (govet, staticcheck, gofmt, forbidigo, depguard, errcheck, unused). The following are the upstream sources those rules derive from — they are pointers, not the operating contract:

- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)

**How** to work in general lives in the neuro-matrix harness; this file says **what** the repo is, plus the repo-specific principles above.
