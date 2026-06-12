# mirabilis greenfield — implementation plan (DRAFT, pre-audit)

**BLUF.** Current mirabilis violates its own TUI framework's architecture (blocking I/O on the UI thread), loses the Claude token on the way into the sandbox, keeps secrets in two half-working stores, and drags devcontainer CLI along as a latency source. We rewrite greenfield in one PR: a pure engine (ports & adapters, zero TUI deps) + Bubble Tea v2 TUI (component tree, fully addressed message bus, persistent menu chrome, command log) + container on docker compose / Docker SDK + a host-side auth proxy (token never leaves the host). Tests are a full pyramid written together with the code. This document is the single source of truth for the implementation (paper-to-code methodology).

---

## 0. How to read this document

- Methodology: [paper-to-code SKILL](https://github.com/AlexShchuka/neuro-matrix/blob/main/skills/paper-to-code/SKILL.md) — "the paper is the only source of truth"; silent gap-filling is forbidden.
- Statement tags:
  - `DECIDED Dn` — fixed by the owner during the 2026-06-12 session (log in §2; verbatim answers in `docs/session-log.md`).
  - `ANCHOR path:line` — fact from current code (commit `531a56b`).
  - `EXTERNAL url` — fact from official documentation / reference repository.
  - `ASSUMPTION` — context inference, reasoning stated inline.
  - `PARTIAL` — known incompletely; resolved at the referenced phase.
  - `QUESTION` — load-bearing gap blocking **only its own node**; that node is not coded until resolved.
- Implementation may not deviate from `DECIDED` without going back to the owner.
- Invariants (§10) and glossary (§13) require the owner's sign-off before the PR merges (AGENTS.md principle 8).

---

## 1. Diagnosis: symptom → root cause → anchor

| # | Symptom (owner's complaint) | Root cause | Anchor |
|---|---|---|---|
| 1 | Entering TG/Claude token → blank screen | huh form in `StateCompleted` renders empty while `apply()` synchronously calls `security add-generic-password`, which blocks on the keychain GUI prompt; UI waits forever | `internal/app/forms.go:270-284`, `internal/runtime/runtime_darwin.go:43-53` |
| 2 | Menu lags, "everything on one thread" | `ComputeStatus` makes 5–6 synchronous `docker`/`devcontainer exec` calls (incl. `claude plugin list` inside the container) and is invoked synchronously before TUI start and inside `Update` on every return to menu | `internal/provision/status.go:20-32`, `internal/app/run.go:18`, `internal/app/app.go:214` |
| 3 | Slow container calls | every container call goes through `devcontainer exec` (Node process, config re-resolution per call) | `internal/runtime/docker.go:16-19` |
| 4 | Claude login "doesn't work, blank screen" | token is injected only via `ComposeEnv` at container **creation**; handoff (`docker exec`) forwards only `GITHUB_PERSONAL_ACCESS_TOKEN`; `IsStale` ignores token appearance → a token entered after the container is up never reaches it. Confirmed on the live container: env lacks `CLAUDE_CODE_OAUTH_TOKEN` | `internal/runtime/env.go:17-44`, `internal/runtime/handoff.go:54-63`, `internal/runtime/docker.go:59-73` |
| 5 | "Says something about password", zombie processes | keychain write has no timeout and runs in the apply path; old versions leave orphans (tinyproxy alive 3.5 days, `security` stuck since 9:40) — no "host process leaves no children" rule | `internal/runtime/runtime_darwin.go:43-53` |
| 6 | Pipeline steps are black boxes | a step has no output stream: only final `RanMsg{Err}`; UI shows a single title line | `internal/pipeline/pipeline.go:187-196,299-313` |
| 7 | No SRP, routing smeared | `appModel.Update` is a manual 4-phase switch; size/esc/ctrl+c forwarding duplicated per branch; all screens' state lives flat in one model | `internal/app/app.go:57-153,229-248` |
| 8 | Dual secret storage | every write goes both to keychain (service name `mirabilis-telegram-token-token` — doubled `-token`) and to a plaintext file; reads come from either | `internal/provision/telegram.go:17-20`, `internal/runtime/env.go:70-86` |
| 9 | Terminal/copy broken after claude | `syscall.Exec` replaces the process: no way back to menu; terminal restore is hand-written escape codes | `internal/runtime/handoff.go:36-52` |
| 10 | Declared security boundary is fiction | `docker-outside-of-docker` feature hands the agent the host docker.sock → escape via `docker run -v /:/host` is trivial, contradicting SECURITY.md | `.devcontainer/devcontainer.json:11` |

`EXTERNAL` confirmation for roots #1–2: Bubble Tea rule — all I/O via `tea.Cmd` only; "accept the architecture or fight it forever" ([Charm: Commands in Bubble Tea](https://charm.land/blog/commands-in-bubbletea/), [habr 939574](https://habr.com/ru/articles/939574/)).

---

## 2. Decision log (all `DECIDED` by owner, 2026-06-12; verbatim in session-log)

| ID | Decision |
|---|---|
| D1 | Greenfield rewrite; old architecture is not an anchor; switch in **one PR** (new packages alongside → entrypoint switch → old code deleted in the same PR) |
| D2 | Business logic is a separate engine **with zero Bubble Tea dependencies**, testable without a terminal |
| D3 | TUI: **component tree + full message bus with addressing** (NodeID, Envelope{To, Msg}) |
| D4 | Pipeline step = **Command{Check, Run→stream, Meta}**; executor is a finite state machine |
| D5 | Framework: **Bubble Tea v2 + bubbles + huh + lipgloss** (Charm stack, already in go.mod) |
| D6 | Claude auth: **interactive login on the host, token stays on the host, never leaks into the sandbox** — host auth proxy (`ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` bearer) |
| D7 | Greenfield scope: **the whole system** (TUI, engine, container, proxy, telegram, handoff, docs, CI) |
| D8 | Tests: **full pyramid** (unit + golden + idempotency contract + smoke), written together with the code |
| D9 | Telegram is an optional step **inside** launch; launch is self-sufficient and **idempotent at every step** |
| D10 | **No comments in code** (repo rule stands); compensation: dense graph, precise names, prose in .md |
| D11 | Handoff: **host process stays alive**, claude is a child via `tea.Exec`; on exit — back to menu |
| D12 | Secrets: **single source of truth** — keychain (macOS) / file 0600 (Linux/WSL), no duplicates; writes only in background and with a timeout |
| D13 | Meta-goal: **minimum code** — dense, complete graph; important, precise nodes (per lazygit/lazydocker/Textual references) |
| D14 | **Nodes are replaceable**: ports & adapters; "another messenger tomorrow" = one new adapter file, not a system edit |
| D15 | Container runtime: **docker compose + Docker Go SDK; devcontainer CLI removed**; claude-code and gh baked into the Dockerfile |
| D16 | docker.sock: **absent by default**, enabled by an explicit flag (changes fingerprint; warning in UI and SECURITY.md) |
| D17 | Claude CLI on the host is a **mandatory dependency**, installed by bootstrap; login without copy-paste |
| D18 | UI language: **English everywhere**, strings in a single node |
| D19 | Auth-proxy spike failure → **STOP and discuss** with spike data; no automatic plan B |
| D20 | Owner's machine untouched; all work within the repository. Environment cleanup is a checklist only (§12) |
| D21 | Plan = "paper" per paper-to-code; catalog forms (stacks/plugins/skills) are pipeline steps too, with Check "choice persisted → skip" (follows D9: "like everything else") |
| D22 | Artifacts: three files — `docs/plan-draft.md` (this), `docs/session-log.md`, `docs/redesign-plan.md` (final, post-audit) |
| D23 | Audit conflicts: load-bearing refutations → STOP and ask owner; minor fixes applied with a note in the final |
| D24 | Independent audit by **three agents** (web facts / code anchors / adversarial architecture), not by the author |
| D25 | After audit: disputed items → AskMe to owner → approval → final file → task ends |
| D26 | All artifact files in **English** to eliminate ambiguity |

---

## 3. Principles

1. **SRP at node level**: a bug localizes to one directory. Logic in `engine`, rendering in `tui`, never mixed (lazygit: `pkg/commands` has zero GUI deps).
2. **SOLID**: dependencies point at ports (interfaces); adapters wired at startup (DIP); nodes open to replacement, closed to modification (OCP).
3. **KISS / minimum code (D13)**: a node exists only if it carries a responsibility no other node has; no "just in case" layers (anti-neuroslop, AGENTS.md).
4. **No black boxes**: every real command is visible in the command log before and during execution (lazygit `LogCommand`).
5. **Idempotency as a contract**: successful `Run` ⇒ immediate `Check` must report satisfied; machine-verified.
6. **Zero daemons**: everything host-side (proxy, watchers) is a goroutine of the mirabilis process; process exit ⇒ no children.
7. **UI thread is sacred**: no exec/fs/network in `Update`/`View` — state and messages only.

---

## 4. Target architecture

### 4.1 Node graph

```
cmd/mirabilis ── dispatch: (no args→tui) | provision --phase | hook | notify send
│
├── internal/bus            message types + addressing (pure Go, no tea import)
│
├── internal/engine         ZERO bubbletea dependencies (D2)
│   ├── exec        port Runner (Host/Container, line streaming) + adapters: dockersdk, fake
│   ├── secrets     port Store + adapters: keychain(darwin), file(linux/wsl)
│   ├── claudeauth  port TokenSource + adapter setuptoken (claude CLI + secrets)
│   ├── authproxy   reverse proxy for api.anthropic.com (host goroutine)
│   ├── sandbox     compose up/build/down/reset, inspect/events(SDK), fingerprint, attach, vscode
│   ├── step        Command contract + Pipeline FSM (deps, optional, retry, timeout, stream)
│   ├── steps       step implementations (§7)
│   ├── notify      port Notifier + adapter telegram (outbox watcher goroutine)
│   ├── membackup   port Save/Restore + adapter dockercp
│   ├── status      state snapshot + docker-events watcher
│   └── config      stacks/plugins/skills catalogs + persisted choices (config/*.txt as today)
│
└── internal/tui            rendering and dispatch only
    ├── app         root: owns frame+router, bridges engine events → bus, holds engine facade
    ├── frame       persistent chrome: header(status), left menu panel, footer(hints)
    ├── router      screen stack in the main area; addressed Envelope delivery
    ├── screens     menu, launch, telegram, harness, reset
    ├── components  steplist, cmdlog, form(huh wrapper), statusbar
    └── strings, styles    all UI strings (EN, D18) and styles — one node each
```

Container side: same binary (`mirabilis provision --phase create|start|plugins|skills`, `mirabilis hook <name>`) — as today (`ANCHOR cmd/mirabilis/main.go:48-56`), steps come from `engine/steps`.

### 4.2 Dependency edges (the complete graph)

Allowed edges (anything not listed is forbidden):

| From | To | Why |
|---|---|---|
| `cmd/mirabilis` | `engine/*` (adapters wiring), `tui/app` | composition root; the only place adapters are constructed |
| `tui/app` | `bus`, engine **ports** (facade), `tui/*` | bridge: engine events → bus msgs; screens' intents → engine calls wrapped in `tea.Cmd` |
| `tui/screens`, `tui/components`, `tui/frame`, `tui/router` | `bus`, `tui/strings`, `tui/styles` | emit/consume messages only; **never** import engine |
| `engine/step` | (nothing inside engine) | pure contract + FSM; deps injected via `Deps` struct |
| `engine/steps` | `exec`, `secrets`, `sandbox`, `config`, `claudeauth`, `notify` ports | step bodies |
| `engine/authproxy` | `claudeauth` (TokenSource) | header injection |
| `engine/claudeauth` | `secrets`, `exec` | token store + setup-token run |
| `engine/sandbox` | `exec`, `config` | lifecycle, fingerprint |
| `engine/status` | `sandbox` | events → snapshots |
| `engine/notify`, `engine/membackup` | `exec`/`secrets` as needed | adapters |
| anything | `os/exec` | **forbidden except** `engine/exec` (linted, invariant I2/I4) |
| `engine/*` | `bus`, `tui/*`, bubbletea | **forbidden** (D2); engine exposes its own event types over channels; `tui/app` converts |

### 4.3 Bus (D3)

`EXTERNAL`: Textual→Bubble Tea mapping ([guide/events](https://textual.textualize.io/guide/events/), [guide/workers](https://textual.textualize.io/guide/workers/)): upward "bubbling" is free in Bubble Tea — any `Cmd`'s Msg reaches the root; addressing is needed for **downward** delivery.

```go
package bus
type NodeID string                          // tree path: "app/launch/steplist"
type Envelope struct{ To NodeID; Msg any }  // addressed downward; To=="" → broadcast

// message families (single registry — the only place types are declared):
// MenuChosen{Action}                screen→root
// StepEvent{Step, Kind, Line}       engine→tui (Kind: started|line|done|failed|skipped|waiting)
// PipelineDone{Failed}
// NeedInteractive{Step}             FSM requests an interactive screen (gh, claude-auth, forms)
// StatusChanged{Snapshot}           status watcher → broadcast
// ScreenPush{Model} / ScreenPop / ScreenResult{Value}
// SecretStored{Key, Err}
```

Rules: (1) upward — a component returns a typed Msg via Cmd; the root sees it first; (2) downward — only Envelope via router; (3) handled ⇒ not re-broadcast (the `event.stop()` analogue); (4) broadcast is reserved for `WindowSize` and `StatusChanged`.

Engine↔tui bridge: engine yields channels of pure events; `tui/app` pumps them with the "one Cmd = one channel read, re-arm after handling" pattern (`ANCHOR` working sample: `internal/ghauth/ghauth.go:69-132`).

### 4.4 TUI: layout and screens

Persistent chrome (D3 + owner's "menu always visible as the wrapper"; reference — lazygit side panels):

```
┌ mirabilis ─────────────────── v1.3.0 · container up · harness on · proxy on ┐
│ > Launch       │ Launch                                                     │
│   Harness      │  ✔ Preflight        docker 28.1, compose ok                │
│   Telegram     │  ✔ Claude auth      token in keychain                      │
│   VS Code      │  ⠋ Container        building image… 02:14                  │
│   Reset        │  · GitHub sign-in   waiting                                │
│   Quit         │  · Provision  · Plugins  · Skills  · Harness               │
│                ├─ commands ──────────────────────────────────────────────── │
│                │ + docker compose up -d --build                             │
│                │ #12 RUN go install honnef.co/...                           │
│                │ + docker exec mirabilis gh auth status                     │
└ enter select · esc back · tab log · q quit ──────────────────────────────────┘
```

- header — live status from `StatusChanged` (pushed by docker events, no polling in Update).
- left panel — menu, always visible; items are objects {title, desc, enabled, action Msg}.
- main — screen from the router stack; cmdlog — viewport component, autoscroll, fed by `StepEvent{Kind:line}` and `Started{Argv}` (anti-black-box, lazygit `LogCommand`).
- interactive subprocess (claude, setup-token) — `tea.Exec`: terminal handed over; on exit UI restored, menu active again (D11).

Screens: `menu` (default), `launch` (steplist+cmdlog), `telegram`, `harness`, `reset` (confirm). VS Code is an action, not a screen.

### 4.5 Step engine (D4, D21)

```go
package step
type Command interface {
    Check(ctx, deps Deps) (satisfied bool, err error)
    Run(ctx, deps Deps, out chan<- Event) error
}
type Meta struct {
    Name, Title string
    Deps        []string
    Optional    bool
    Interactive bool          // FSM emits NeedInteractive and awaits ScreenResult
    Retry       RetryPolicy
    Timeout     time.Duration
}
```

The FSM keeps the current pipeline's semantics (deps/optional/cascade-skip — `ANCHOR internal/pipeline/pipeline.go:230-297`) and adds: `out` streaming to the bridge; unified interactive steps (gh, claude-auth, catalog forms, telegram — one mechanism instead of the `NeedGHMsg` special case); ctx-cancellation on `esc`.

Idempotency contract: after successful `Run`, an immediate `Check` must return `true` (test §8.2). A repeat launch on a healthy system = all Checks satisfied = zero questions, zero actions (D9, invariant I9).

### 4.6 Auth proxy (D6)

`EXTERNAL` [code.claude.com/docs/en/authentication](https://code.claude.com/docs/en/authentication): proxy/gateway is an official path; `ANTHROPIC_AUTH_TOKEN` is sent as `Authorization: Bearer`; `ANTHROPIC_BASE_URL` redirects the client.

Spec:
- Listens on the host; container reaches it via `host.docker.internal` (`extra_hosts: host-gateway` already in compose — `ANCHOR docker-compose.yml:27-28`). `PARTIAL`: Linux listen interface (loopback vs bridge IP) — resolved by spike Ph0.
- Per-session key: random on each host-process start. **Neither the real token nor the key is written into container env at creation** — passed per-exec only: `docker exec -e ANTHROPIC_BASE_URL -e ANTHROPIC_AUTH_TOKEN=<session-key> … claude`. Consequences: token rotation needs no container re-create; claude invoked outside mirabilis is unauthenticated (feature: token unusable without the live host process); port/key may change freely between sessions.
- Proxy validates the session key, swaps Authorization for the real Bearer from `TokenSource`, transparently streams SSE. `PARTIAL`: necessity of the `anthropic-beta` oauth header — spike.
- The token is never logged (log-redaction test).
- `QUESTION Q1` (blocks authproxy+claudeauth): behaviour of a subscription oat token through a Bearer proxy. Resolved by spike Ph0. Failure → **STOP and discuss** (D19).

### 4.7 Container (D15, D16)

- Dockerfile: + `npm install -g @anthropic-ai/claude-code@<pin>` and gh CLI (apt, pinned) — moved from devcontainer features (`ANCHOR .devcontainer/devcontainer.json:8-12`; versions from `devcontainer-lock.json`); the rest unchanged.
- compose: feature lifecycle hooks replaced — host side runs `provision --phase create/start` after `up` as a pipeline step (visible in cmdlog); docker.sock — separate compose profile `docker-sock`, off by default (D16); enabling it flips the fingerprint → re-create.
- `sandbox.fingerprint` = git-short + STACKS + sock flag (replaces `IsStale`; token excluded — no longer in env, §4.6).
- Docker Go SDK: inspect/exec/events/cp (lazydocker pattern); `compose up/build/down` via CLI (compose v2 has no SDK). Docker events → `status` → push header updates.
- devcontainer CLI: removed from bootstrap/Makefile/install.sh. `QUESTION Q4` (blocks only the .devcontainer file): delete `devcontainer.json` entirely or keep a minimal one for VS Code UX? Default `ASSUMPTION` — delete (`vscode.go` already uses the `attached-container` URI: `ANCHOR internal/runtime/vscode.go:26-28`); finalized at PR review.

### 4.8 Host process lifecycle (D11)

```
mirabilis started ── goroutines: authproxy, status watcher (docker events), notify watcher (if configured)
   └─ TUI loop ── launch pipeline ── tea.Exec(docker exec … claude) ── back to menu
mirabilis exited ── all goroutines die with the process; no children (invariant I5)
```

The `tg-outbox` subcommand disappears (`ANCHOR cmd/mirabilis/main.go:62-85` — was a separate process); the watcher becomes a goroutine.

---

## 5. Flows

**First launch**: menu → Launch → pipeline: preflight ✔ → claude-auth (Check: no token → `tea.Exec claude setup-token`, parse from pty, `Store.Set` in background) → stacks/plugins/skills forms (Check: no persisted choice → show) → telegram (Check: not configured and not explicitly skipped → form; "Skip" is persisted) → image build (streamed to cmdlog) → container up → provision create/start → gh-auth (device flow, code+URL on screen) → plugins/skills/harness → `tea.Exec claude` → exit → menu.

**Repeat launch**: all Checks satisfied → pipeline flies green in seconds → straight to claude (I9).

**Adapter swap** (D14): file `internal/engine/notify/slack.go` + one registration line. Nothing else.

**Reset**: confirm screen → `membackup.Save` (if preserve) → `sandbox.Reset` → menu with notice.

---

## 6. Anchor map: old node → fate

| Current (`531a56b`) | Fate | Destination |
|---|---|---|
| `cmd/mirabilis/main.go` | rewritten; dispatch kept; `tg-outbox` → goroutine | `cmd/mirabilis` |
| `cmd/tgsend`, `cmd/tgsmoke` | folded into `notify send` subcommand / adapter tests. `ASSUMPTION`: usage inside hooks to be confirmed from `internal/hooks/hooks.go` at Ph4 | `engine/notify` |
| `internal/app/{app,menu,forms,run}.go` | **replaced** (§1 #1,2,7) | `tui/*` |
| `internal/pipeline` | replaced; deps/optional/retry semantics carried over; streaming added | `engine/step` |
| `internal/steps/*` | rewritten as `Command` with streaming; Check/Run logic carried over | `engine/steps` |
| `internal/runner` | Fake idea kept (lazygit `FakeCmdObjRunner`) | `engine/exec` |
| `internal/runtime/docker.go` | exec→SDK; `IsStale`→fingerprint; `SaveMemory`→membackup | `engine/sandbox`, `engine/membackup` |
| `internal/runtime/env.go` | dies: env assembly → sandbox; keychain reads → secrets | — |
| `internal/runtime/handoff.go` | `syscall.Exec` → `tea.Exec` attach (§1 #9) | `engine/sandbox` |
| `internal/runtime/runtime_darwin.go` | keychain → secrets adapter; entry names fixed (§1 #8); writes with timeout | `engine/secrets` |
| `internal/runtime/vscode.go` | carried over nearly as-is | `engine/sandbox` |
| `internal/provision/{telegram,claude}.go` | secrets → secrets/claudeauth; dual-write removed (D12) | `engine/secrets`, `engine/claudeauth` |
| `internal/provision/status.go` | sync snapshot → docker-events watcher + lazy container probes off the UI thread (§1 #2) | `engine/status` |
| `internal/provision/*` (rtk, settings, memory, hud, mcp, harness, plugins, skills, statefile, gitidentity) | in-container phases carried over (working logic), shaped as steps | `engine/steps` |
| `internal/ghauth` | carried over (streaming already correct); special Msg → generic `NeedInteractive` | `engine/steps/ghauth` + screen |
| `internal/telegram`, `internal/tgtoken` | behind the Notifier port; tgtoken dies into secrets | `engine/notify` |
| `internal/hooks` | stays (container side), writes to the notify outbox | `engine/notify` or own node — decided when read at Ph4 |
| `internal/ui` | strings EN-only (D18), styles | `tui/strings`, `tui/styles` |
| `internal/config` | carried over | `engine/config` |
| `.devcontainer/*` | features → Dockerfile; json — Q4 | minimal `.devcontainer` or deleted |
| `docker-compose.yml` | + `docker-sock` profile; secret env vars at create removed (per-exec instead) | same file |
| `Makefile`, `install.sh`, `Brewfile` | − devcontainer CLI; + claude CLI (D17); `up` = compose | same files |
| `AGENTS.md`, `SECURITY.md`, `README.md` | revision per §11 | same files |
| `test/*.bats`, golden files | kept and extended | `test/`, `tui/testdata` |

Completeness: every existing package is accounted for; no orphan nodes remain (invariant I11).

---

## 7. Pipeline step registry (target)

| Step | Check (idempotency) | Run | Kind |
|---|---|---|---|
| preflight | docker reachable, compose file valid | start Docker Desktop (darwin) / print hint | auto |
| claude-auth | `TokenSource.Token()` present | `tea.Exec claude setup-token` + parse + `Store.Set` | interactive |
| stacks/plugins/skills | choice persisted | multiselect form, write config | interactive |
| telegram | configured **or** explicitly skipped | select+token form; `Store.Set` in background with timeout | interactive, optional |
| image | fingerprint matches | `docker compose build` (streamed to cmdlog) | auto |
| container | running && fingerprint ok | `compose up -d`, wait healthy | auto |
| proxy | proxy listening, session key generated | start goroutine | auto |
| provision-create/start | statefile inside container | `docker exec mirabilis provision --phase …` | auto |
| gh-auth | `gh auth status` ok in container | device flow with streaming | interactive |
| plugins / skills / harness | container state matches choice | install/remove | auto, optional |
| attach | — (terminal) | `tea.Exec docker exec … claude` + system-prompt assembly (`ANCHOR internal/runtime/handoff.go:76-79`) | terminal |

---

## 8. Tests (D8)

1. **Engine unit** — every node against `exec.Fake` (lazygit ExpectArgs pattern); secrets file backend; bus routing (address, broadcast, stop).
2. **Idempotency contract** — table test over the step registry: `Run(fake)` ⇒ `Check==true`; a step failing the contract cannot be registered.
3. **Golden TUI** — teatest v2 (already in go.mod): menu, launch progress, cmdlog, forms; successor of `TestFlowMenuGolden.golden`.
4. **authproxy** — httptest upstream: Authorization injection, refusal without session key, token absent from logs, SSE pass-through.
5. **Smoke** — `test/install.bats`, `pre_commit.bats` kept; + bats check of `mirabilis --help` / `provision --phase` dispatch.
6. **CI** — matrix ubuntu+macos: `go test ./...`, golangci-lint, bats. Keychain tests skipped in CI (`ANCHOR` current guard: `internal/runtime/runtime_darwin.go:32`).
7. **Manual e2e checklist** (in the PR): clean machine → install.sh → mirabilis → full first launch → exit claude → menu → repeat launch < 10 s → reset.

---

## 9. Implementation phases (one PR, sequential commits)

| Phase | Content | Definition of Done |
|---|---|---|
| **0. Auth-proxy spike** | minimal proxy under `test/spike` (deleted at PR end): oat token on host, container with BASE_URL+session-key, `claude -p "ping"` | model reply + streaming work; `/status` shows auth ok; listen interface and beta header recorded as facts here. **Failure → STOP, discuss (D19)** |
| 1. Foundation | `bus`, `engine/exec`(+fake), `engine/secrets`, `engine/config` | units green; keychain entry names without doubles; secret write with timeout |
| 2. Sandbox | SDK client, compose wrapper, fingerprint, events, `engine/status` | container up/detect tested with fake; events → snapshot |
| 3. Steps | `engine/step` FSM + non-interactive steps | idempotency contract green for all |
| 4. Services | `authproxy`, `claudeauth`, gh-auth, `notify` (telegram + watcher goroutine), `membackup`; read `internal/hooks`, close Q2 | proxy tests green; Q2 resolved as a fact in this doc |
| 5. TUI | frame, router, components, screens; engine-event bridge | goldens green; manual check: no blocking calls in Update (review + I2) |
| 6. Switch | `cmd/mirabilis` onto new nodes; Dockerfile/compose/Makefile/install.sh/Brewfile; **delete** old packages; AGENTS/SECURITY/README | repo holds no dead code; CI green |
| 7. Verification | e2e checklist, invariants §10 executed, owner sign-off | PR ready for review |

---

## 10. Invariants (sign-off table for the owner)

| ID | Invariant | Verification |
|---|---|---|
| I1 | Real Anthropic token never present in the container: not in env (create or exec), not on fs, not in the image | e2e: `docker inspect` + env dump in exec; image grep |
| I2 | UI thread does no I/O: `Update`/`View` never exec/fs/network | forbidigo: `os/exec` allowed only in `engine/exec`; review |
| I3 | Every step idempotent: Run ⇒ Check=true | contract test §8.2 |
| I4 | Every command visible: process spawn only via Runner; Runner always emits Started{Argv} to cmdlog | forbidigo + bus unit |
| I5 | Host process leaves no children after exit | e2e: ps after quit |
| I6 | Port adapter swap = new file + registration; zero edits elsewhere | structure review; demo adapter in tests |
| I7 | A secret lives in exactly one backend per platform | secrets unit; no second write path |
| I8 | All UI strings in `tui/strings`, English | grep test |
| I9 | Repeat launch on a healthy system: zero questions, zero changes | e2e checklist |
| I10 | docker.sock absent by default; enabling changes fingerprint | fingerprint unit + e2e inspect mounts |
| I11 | §6 map covers all old nodes; no dead code after switch | PR diff review |

## 11. Document edits

- **AGENTS.md**: Layout → new graph; Boundaries → "open egress stays; api.anthropic.com goes through the host auth broker; the secret never enters the sandbox"; remove devcontainer CLI mentions.
- **SECURITY.md**: threat model with the proxy and without the socket; section on consciously enabling the `docker-sock` profile.
- **README.md**: quick start (bootstrap installs claude CLI), single secret flow, new TUI screenshot.

## 12. Environment cleanup checklist (owner-executed, outside repo scope — D20)

1. `kill 89506` — orphaned tinyproxy (from the removed version; alive 3.5 days).
2. `kill 68036` — stuck `security add-generic-password` (holds the neighbouring terminal).
3. `security delete-generic-password -s mirabilis-telegram-token-token` — entry with the old doubled name (after the new naming ships).
4. Decide the fate of the repo checkout inside `$HOME` (cmd/, internal/, go.mod at the home root) — looks accidental.
5. `rm /var/folders/.../mirabilis-proxy.{log,conf}` — 6 MB tinyproxy log.

## 13. Glossary

- **Node** — a package with one responsibility; the unit of replacement and bug localization.
- **Port / adapter** — a node's interface / its swappable implementation (D14).
- **Bus** — the message-type registry + delivery rules (upward via Cmd, downward via Envelope).
- **Step (Command)** — an idempotent pipeline unit with Check/Run/Meta and output streaming.
- **Fingerprint** — deterministic digest of the desired container configuration; mismatch ⇒ re-create.
- **Session key** — one-time auth-proxy access key; the only "secret" the container sees, useless outside the proxy.
- **cmdlog** — the panel of actually executed commands and their output (anti-black-box).

## 14. Open QUESTIONs

| ID | Question | Blocks | Resolution |
|---|---|---|---|
| Q1 | Subscription oat token through a Bearer proxy: accepted by the API? beta header needed? streaming stable? | authproxy, claudeauth | spike Ph0; failure → STOP (D19) |
| Q2 | How hooks use tgsend (container→host queue contract) | notify, hooks | read `internal/hooks/hooks.go` at Ph4; record fact here |
| Q3 | Proxy listen interface on Linux (loopback vs bridge) | authproxy (linux) | spike Ph0 |
| Q4 | `.devcontainer/devcontainer.json`: delete or keep minimal | that file only | PR review; default — delete |
