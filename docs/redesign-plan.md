# mirabilis greenfield — implementation plan (FINAL)

**BLUF.** Current mirabilis violates its own TUI framework's architecture (blocking I/O on the UI thread), loses the Claude token on the way into the sandbox, keeps secrets in two half-working stores, runs a second hidden in-container proxy that fights the intended design, and drags devcontainer CLI along as a latency source. We rewrite greenfield in one PR around a system-first architecture: a pure engine (ports & adapters, zero TUI deps) + Bubble Tea v2 TUI (component tree, addressed message bus, persistent menu chrome, command log) + container on docker compose / Docker Go SDK + a host-side auth proxy chained behind the in-container observability proxy so the real token never leaves the host. Tests are a full pyramid (unit → snapshot → e2e → user-scenario) written together with the code. This document is the single source of truth for implementation (paper-to-code methodology). It supersedes `plan-draft.md`; the draft and `session-log.md` remain as the decision trail.

---

## 0. Meta-goals (priority order — these outrank fixing any single node)

These are the owner's standing directives (2026-06-12). When a node-level convenience conflicts with one of these, the meta-goal wins.

- **G0 System over nodes.** A scalable, reliable, flexible system is the goal; good architecture *makes* node repair easy as a consequence. The reverse (patch nodes, hope architecture emerges) is what produced today's mess. Every decision is judged by what it does to the whole graph first.
- **G1 Human- and robot-readable architecture.** Dense, compressed, coherent, contradiction-free; no neuroslop, no invented abstractions, no dangling leaves, no code-for-code's-sake. 2026 engineering practice, no crutches/reinvented wheels. Fewer, more precise nodes beat more code (LLMs read a tight graph better than a sprawling one — and we keep the repo's no-comments rule, so the graph itself must carry the meaning).
- **G2 Sandbox ≠ harness.** The container is a deployment+runtime vehicle for tools and the `neuro-matrix` harness. The harness executes behaviour and policy; mirabilis owns **provisioning, autonomy, reproducibility, thread-safety, speed**. Behavioural logic must not leak into sandbox nodes.
- **G3 Separation of responsibility (SRP).** One responsibility per node; a bug localizes to one directory. Logic in `engine`, rendering in `tui`, never mixed.
- **G4 Configuration separate from logic.** All tunables (stacks/plugins/skills, sock flag, proxy port, adapter selection) live in `config/`; code only reads them. Behaviour changes without editing code.
- **G5 Observability in one place.** Structured logs (slog) and the status of every node converge on one sink: a single host log file + the TUI status header, both fed by the same event stream. Nothing writes "into the void".
- **G6 Graceful degradation.** A node's failure degrades its function, not the system. Optional steps cascade-skip; notify/status/proxy failure shows a `degraded` status and the menu keeps working; goroutines recover; every external call has a timeout (so a keychain stall becomes a step error, never a UI hang).
- **G7 Idempotency & reproducibility everywhere.** Every operation (not just steps) is safe to repeat and deterministic: a fingerprint reproduces the container, `Check→Run→Check` is the step contract, same input → same result. This is also the primary test lever (run twice, assert the second pass is a no-op).
- **G8 Replaceable nodes.** Ports & adapters: "swap Telegram for another messenger tomorrow" / "another memory-backup" / "another Claude-login method" = one new adapter file + one registration line, zero edits elsewhere.

---

## 1. How to read this document

- Methodology: [paper-to-code SKILL](https://github.com/AlexShchuka/neuro-matrix/blob/main/skills/paper-to-code/SKILL.md) — "the paper is the only source of truth"; silent gap-filling is forbidden.
- Statement tags:
  - `DECIDED Dn` — fixed by the owner (session 2026-06-12; verbatim in `session-log.md`).
  - `ANCHOR path:line` — fact from current code (commit `531a56b`), independently re-verified by the code-anchor audit (15/15 confirmed).
  - `EXTERNAL url` — fact from official docs / reference repo, independently re-verified by the web-fact audit.
  - `AUDIT` — change introduced by the independent audit (web / code / architecture).
  - `ASSUMPTION` — context inference, reasoning inline.
  - `PARTIAL` — known incompletely; resolved at the referenced phase.
  - `QUESTION` — load-bearing gap blocking **only its own node**; that node is not coded until resolved.
- Implementation may not deviate from `DECIDED` without returning to the owner.
- Invariants (§10) and glossary (§13) require the owner's sign-off before merge (AGENTS.md principle 8).

---

## 2. Diagnosis: symptom → root cause → anchor

| # | Symptom (owner) | Root cause | Anchor |
|---|---|---|---|
| 1 | Token entry → blank screen | huh form at `StateCompleted` renders empty (`if f.quitting { return "" }`) while `apply()` synchronously calls `security add-generic-password`, which blocks on the keychain GUI prompt; UI waits forever | `internal/app/forms.go:270-284`, `internal/runtime/runtime_darwin.go:43-53` |
| 2 | Menu lags, "single-threaded" | `ComputeStatus` makes 6+ synchronous `docker`/`devcontainer exec` calls (incl. `claude plugin list` in container) and runs synchronously pre-TUI and inside `Update` on every menu return | `internal/provision/status.go:20-32`, `internal/app/run.go:18`, `internal/app/app.go:214` |
| 3 | Slow container calls | every container call goes through `devcontainer exec` (Node process, config re-resolution per call) | `internal/runtime/docker.go:16-19` |
| 4 | Claude login broken, blank screen | token injected only via `ComposeEnv` at container **create**; handoff (`docker exec`) forwards only `GITHUB_PERSONAL_ACCESS_TOKEN`; `IsStale` ignores token; a token entered after the container is up never reaches it. Confirmed live: container env lacks `CLAUDE_CODE_OAUTH_TOKEN` | `internal/runtime/env.go:17-44`, `internal/runtime/handoff.go:54-63`, `internal/runtime/docker.go:59-73` |
| 5 | "Says password", zombies | keychain write has no timeout, runs in the apply path; old versions orphan processes (live: tinyproxy 3.5 days, stuck `security` since 9:40) — no "host process leaves no children" rule | `internal/runtime/runtime_darwin.go:43-53` |
| 6 | Pipeline steps are black boxes | a step has no output stream: only final `RanMsg{Err}`; UI shows one title line | `internal/pipeline/pipeline.go:187-196,299-313` |
| 7 | No SRP, routing smeared | `appModel.Update` is a manual 4-phase switch; size/esc/ctrl+c forwarding duplicated per branch; all screens' state flat in one model | `internal/app/app.go:57-153,229-248` |
| 8 | Dual secret storage | every write goes to keychain (service name `mirabilis-<key>-token` — doubled `-token`) **and** a plaintext file; reads come from either | `internal/provision/telegram.go:17-20`, `internal/runtime/env.go:70-86`, `internal/runtime/runtime_darwin.go:24,46` |
| 9 | Terminal/copy broken after claude | `syscall.Exec` replaces the process: no way back to menu; terminal restore is hand-written escapes | `internal/runtime/handoff.go:36-52` |
| 10 | Security boundary is fiction | `docker-outside-of-docker` feature hands the agent the host docker.sock → escape via `docker run -v /:/host` is trivial, contradicting SECURITY.md | `.devcontainer/devcontainer.json:11` |
| 11 | **Hidden second proxy** `AUDIT` | `EnsureHeadroom`/`EnsureHeadroomProxy` install headroom-ai in-container and write `ANTHROPIC_BASE_URL=http://127.0.0.1:8787` into the container's `settings.json` (both provision phases); the SessionStart hook re-arms it every session. This silently collides with the intended host auth proxy | `internal/provision/headroom.go:12,42-53,71-88`, `internal/provision/provision.go:66-67`, `internal/hooks/hooks.go:317-340` |

`EXTERNAL` confirmation for roots #1–2: Bubble Tea — all I/O via `tea.Cmd` only; "Use commands for all I/O" ([Charm: Commands in Bubble Tea](https://charm.land/blog/commands-in-bubbletea/)); "accept the architecture or fight it" ([habr 939574](https://habr.com/ru/articles/939574/)). huh empty-render at completion verified in [huh form.go](https://pkg.go.dev/charm.land/huh/v2).

---

## 3. Decision log

All `DECIDED` by the owner, session 2026-06-12 (verbatim answers in `session-log.md`).

| ID | Decision |
|---|---|
| D1 | Greenfield rewrite; old architecture is not an anchor; switch in **one PR** |
| D2 | Engine has **zero Bubble Tea dependencies**, testable without a terminal |
| D3 | TUI: **component tree + addressed message bus** (NodeID, Envelope{To, Msg}) |
| D4 | Step = **Command{Check, Run→stream, Meta}**; executor is an FSM |
| D5 | Framework: **Bubble Tea v2 + bubbles + huh + lipgloss** (already in go.mod) |
| D6 | Claude auth: **interactive host login, token stays on host, never leaks into the sandbox** — host auth proxy (`ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` bearer) |
| D7 | Scope: **the whole system** (TUI, engine, container, proxy, telegram, handoff, docs, CI) |
| D8 | Tests: **full pyramid** (unit + snapshot + e2e + user-scenario), written with the code |
| D9 | Telegram is an optional step **inside** launch; launch is self-sufficient and **idempotent at every step** |
| D10 | **No comments in code** (repo rule stands); compensation: dense graph, precise names, prose in .md |
| D11 | Handoff: **host process stays alive**, claude is a child via `tea.Exec`; on exit → back to menu |
| D12 | Secrets: **single source of truth** — keychain (macOS) / file 0600 (Linux/WSL), no duplicates; writes only in background, with a timeout |
| D13 | Meta-goal **minimum code** (see G1) |
| D14 | **Replaceable nodes** via ports & adapters (see G8) |
| D15 | Container runtime: **docker compose + Docker Go SDK; devcontainer CLI removed**; claude-code & gh baked into the Dockerfile |
| D16 | docker.sock: **absent by default**, enabled by an explicit flag (changes fingerprint; UI + SECURITY.md warning) |
| D17 | Host claude CLI is a **mandatory dependency** (bootstrap installs it); login without copy-paste |
| D18 | UI language: **English everywhere**, strings in a single node |
| D19 | Auth-proxy spike failure → **STOP and discuss**; no silent plan B |
| D20 | Owner's machine untouched; repo-only work. Cleanup is a checklist (§12) |
| D21 | Catalog forms (stacks/plugins/skills) are pipeline steps too, Check "choice persisted → skip" |
| D22 | Artifacts: `plan-draft.md`, `session-log.md`, `redesign-plan.md` (this final) |
| D23 | Audit conflicts: load-bearing → STOP+ask owner; minor → fix with a note |
| D24 | Independent audit by **three agents** (web facts / code anchors / adversarial architecture) |
| D25 | After audit: disputed items → AskMe → approval → final file → task ends |
| D26 | All artifact files in **English** to remove ambiguity |
| D27 | `AUDIT` Headroom resolution: **chain** claude → headroom (`:8787`, observability + MCP) → host auth proxy (token injection) → Anthropic. Each node one role; token stays off the container |
| D28 | `AUDIT` Auth risk: keep host proxy **primary**; record Q1 risk explicitly and **log alternatives** (§14, §15); spike + STOP per D19 still gates it |

Meta-goals G0–G8 (§0) are owner directives of the same authority as D1–D28 and take precedence on conflict (G0).

---

## 4. Target architecture

### 4.1 Node graph

```
cmd/mirabilis ── dispatch: (no args→tui) | provision --phase | hook | notify send
│
├── internal/bus            message types + addressing (pure Go, no tea import)
├── internal/obs            slog sink + status registry (G5): one log file, node-status aggregation
│
├── internal/engine         ZERO bubbletea dependencies (D2/G2)
│   ├── exec        port Runner (Host/Container, line streaming) + adapters: dockersdk, fake
│   ├── secrets     port Store + adapters: keychain(darwin), file(linux/wsl); one-time migration (§4.9)
│   ├── claudeauth  port TokenSource + adapter setuptoken (pty-interposer, §4.6)
│   ├── authproxy   reverse proxy for api.anthropic.com (host goroutine, started at boot)
│   ├── sandbox     compose up/build/down/reset, inspect/events(SDK), fingerprint, attach, vscode
│   ├── pipeline    Command contract + FSM (deps, optional, retry, timeout, stream, Resume)
│   ├── steps       host launch-step implementations (§7)
│   ├── provision   container-side provision steps (create|start|plugins|skills), same Command contract (AUDIT-IMPL 2026-06-12: split from steps — the two registries run in different processes; §6 internal/provision rows land here)
│   ├── notify      port Notifier + adapter telegram (host-side; outbox watcher goroutine; chat-id host-side)
│   ├── membackup   port Save (host dockercp); Restore stays a provision sub-step (§4.8, finding-7 resolution)
│   ├── status      container inspect snapshot + docker-events resubscribe loop
│   └── config      stacks/plugins/skills catalogs + persisted choices + tunables (G4)
│
└── internal/tui            rendering and dispatch only
    ├── app         root: owns frame+router, bridges engine events↔bus, holds engine facade, runs tea.Exec
    ├── frame       persistent chrome: header(status from obs), left menu panel, footer(hints)
    ├── router      screen stack in main area; addressed Envelope delivery
    ├── screens     menu, launch, telegram, harness, reset
    ├── components  steplist, cmdlog, form(huh wrapper), statusbar
    └── strings, styles    all UI strings (EN, D18) and styles — one node each
```

Container side (same binary): `provision --phase create|start|plugins|skills`, `hook <name>` — as today (`ANCHOR cmd/mirabilis/main.go:48-56`); step bodies come from `engine/steps`.

### 4.2 Dependency edges (the complete graph)

Anything not listed is forbidden (lint-enforced where mechanical).

| From | To | Why |
|---|---|---|
| `cmd/mirabilis` | `engine/*`, `tui/app`, `obs` | composition root; the only place adapters are constructed/registered (G8) |
| `tui/app` | `bus`, engine **ports** (facade), `obs`, `tui/*` | bridge: engine events → bus msgs; intents → engine calls wrapped in `tea.Cmd`; runs `tea.Exec` (D11) |
| `tui/{screens,components,frame,router}` | `bus`, `tui/strings`, `tui/styles` | messages only; **never** import engine |
| `engine/pipeline` | `obs` | pure contract + FSM; step deps injected via `Deps` |
| `engine/steps` | `exec`, `secrets`, `sandbox`, `config`, `claudeauth`, `notify`, `membackup`, `obs` | step bodies |
| `engine/authproxy` | `claudeauth`, `obs` | header injection |
| `engine/claudeauth` | `secrets`, `exec`, `obs` | token store + setup-token |
| `engine/secrets` | `exec` | `AUDIT-IMPL 2026-06-12`: edge was missing — the keychain adapter spawns `security` via the Runner port (I2/I4 forbid raw `os/exec`); file adapter is pure stdlib |
| `bus` | `obs` | `AUDIT-IMPL 2026-06-12`: edge was missing — `StatusChanged` carries `obs.Snapshot` (consistent with finding 11: obs is the one both-layer node) |
| `engine/sandbox` | `exec`, `config`, `obs` | lifecycle, fingerprint |
| `engine/{status,notify,membackup}` | `exec`/`secrets`/`sandbox` as needed, `obs` | adapters |
| any | `os/exec` | **forbidden except** `engine/exec` (lint I2/I4) |
| `engine/*` | `bus`, `tui/*`, bubbletea | **forbidden** (D2/G2); engine emits its own event types over channels; `tui/app` converts to `bus` |

`AUDIT` (finding 11): `obs` is the only node both layers may import — it holds plain status/log types, no behaviour, so it does not breach the engine↔tui wall.

### 4.3 Bus and the engine↔tui bridge (D3)

`EXTERNAL` Textual→Bubble Tea mapping ([guide/events](https://textual.textualize.io/guide/events/), [guide/workers](https://textual.textualize.io/guide/workers/)): upward "bubbling" is free — any `Cmd`'s Msg reaches the root; addressing is only needed downward.

```go
package bus
type NodeID string                          // tree path: "app/launch/steplist"
type Envelope struct{ To NodeID; Msg any }  // addressed downward; To=="" → broadcast

// bus message families (the only place BUS types are declared):
// MenuChosen{Action}
// StepEvent{Step, Kind, Line}        Kind: started|line|done|failed|skipped|waiting
// PipelineDone{Failed}
// NeedInteractive{Step, Payload any} Payload = plain data (catalog options, prompt) fetched by app
// NeedsTerminal{Step, Argv}          step needs the real terminal; app runs tea.Exec, then Resume
// ScreenPush{Model any} / ScreenPop / ScreenResult{Value}
// StatusChanged{Snapshot}            from obs → broadcast
// SecretStored{Key, Err}
```

Rules: (1) upward — a component returns a typed Msg via Cmd; the root sees it first; (2) downward — only Envelope via router; (3) handled ⇒ not re-broadcast (`event.stop()` analogue); (4) broadcast reserved for `WindowSize` and `StatusChanged`.

`AUDIT` (finding 2, 11) — **the bridge is bidirectional and is the only crossing**:
- **Outbound** (engine→ui): engine yields channels of pure event structs; `tui/app` pumps them "one Cmd = one read, re-arm after handling" (`ANCHOR` working sample: `internal/ghauth/ghauth.go:69-132`), converting each to a `bus` Msg. Engine event types are declared in `engine/*`; `bus` mirrors are distinct types — §4.3's "single registry" means *bus* types only.
- **Inbound** (ui→engine): the pipeline handle exposes `Resume(step string, r Result)` where `Result` has a `Cancelled` variant. `tui/app` converts `ScreenResult`→`Resume(step, value)` and `ScreenPop`-without-result→`Resume(step, Cancelled)`. This is the *only* way an interactive step continues; there is no deadlock because a popped screen always yields `Cancelled`.
- Interactive payloads (catalog options, prompts) are plain structs fetched by `tui/app` from the engine facade and passed into screen constructors — screens still never import engine.

### 4.4 TUI: layout and screens

Persistent chrome (D3 + owner's "menu always visible as the wrapper"; reference — lazygit side panels & command log):

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
└ enter select · esc back · tab log · q quit · degraded: notify ───────────────┘
```

- header — live status from `StatusChanged` (pushed by `obs`, fed by docker events; **no polling in Update**); a failed node shows `degraded: <node>` (G6).
- left panel — menu, always visible; items are objects {title, desc, enabled, action Msg}.
- main — screen from router stack; cmdlog — viewport, autoscroll, fed by `StepEvent{Kind:line}` and `started{Argv}` (anti-black-box; lazygit `LogCommand`). cmdlog and the `obs` log file share one source: every Runner spawn emits `started{Argv}` (I4).
- interactive subprocess (claude, setup-token) — `tea.Exec` from `tui/app`: terminal handed over; on exit UI restored, menu active (D11).

Screens: `menu` (default), `launch` (steplist+cmdlog), `telegram`, `harness`, `reset` (confirm). VS Code is an action.

### 4.5 Step engine (D4, D21, G7)

```go
package pipeline
type Command interface {
    Check(ctx, deps Deps) (satisfied bool, err error)
    Run(ctx, deps Deps, out chan<- Event) error
}
type Kind int   // Auto | Interactive | Terminal
type Meta struct {
    Name, Title string
    Deps        []string
    Optional    bool
    Kind        Kind          // Interactive → emits NeedInteractive, awaits Resume; Terminal → emits NeedsTerminal
    Retry       RetryPolicy
    Timeout     time.Duration
}
```

The FSM keeps current semantics (deps/optional/cascade-skip — `ANCHOR internal/pipeline/pipeline.go:230-297`) and adds: `out` streaming; unified interactive/terminal steps via `Resume` (replaces the `NeedGHMsg` special case); ctx-cancellation on `esc`.

`AUDIT` (finding 2): **Terminal/Interactive steps do not call `tea.Exec` themselves** (that would breach D2/G2). They emit `NeedsTerminal{Argv}` / `NeedInteractive{Payload}`; `tui/app` performs `tea.Exec` or shows the screen, then calls `Resume`. `Meta.Timeout` is suspended while a step is awaiting `Resume` (the TUI loop owns the wall-clock then).

Idempotency contract (G7): after a successful `Run`, an immediate `Check` must return `true`. A repeat launch on a healthy system = all Checks satisfied = zero questions, zero actions (D9, I9).

### 4.6 Auth proxy, chained behind headroom (D6, D27)

`EXTERNAL` [code.claude.com/docs/en/authentication](https://code.claude.com/docs/en/authentication): proxy/gateway is an official path; `ANTHROPIC_AUTH_TOKEN` → `Authorization: Bearer`, precedence rank 2 (beats everything else present in the container); `ANTHROPIC_BASE_URL` redirects the client; `claude setup-token` prints a ~1-year oat token (`sk-ant-oat01-…`) and does not save it.

Topology (D27): **claude → headroom (`127.0.0.1:8787`, in-container, observability + MCP) → host auth proxy (`host.docker.internal:<port>`, injects the real Bearer) → api.anthropic.com.** Each node has one role; the real token lives only in the host proxy.

Spec:
- `settings.json` keeps `ANTHROPIC_BASE_URL=http://127.0.0.1:8787` (headroom). Headroom's **upstream** is configured (at provision-start) to the host proxy URL, not Anthropic directly. `AUDIT-IMPL 2026-06-12` resolved (was `PARTIAL`): headroom-ai 0.24.0 upstream key is env `ANTHROPIC_TARGET_API_URL` / flag `--anthropic-api-url` (verified in-container: `providers/registry.py:104` → `proxy/server.py:319,3389`); headroom forwards the client `Authorization` header upstream untouched, stripping only `x-headroom-*` (`proxy/handlers/anthropic.py:660-676`), so the session key reaches the host proxy.
- Host proxy listens on the host; container reaches it via `host.docker.internal` (`extra_hosts: host-gateway` already present — `ANCHOR docker-compose.yml:27-28`). `EXTERNAL`: `host-gateway` is documented on the `docker run --add-host` reference and passed through by compose at runtime (not at build time — [docker/compose#9768](https://github.com/docker/compose/issues/9768)); resolves by default on Docker Desktop macOS. `PARTIAL Q3`: Linux listen interface (loopback vs bridge IP) — spike Ph0.
- Per-session key: random per host-process start, passed to headroom's upstream auth (so the container ever holds only a key useless without the live host proxy). The **real token never enters container env** (I1).
- The host proxy validates the session key, swaps Authorization for the real Bearer from `TokenSource`, transparently streams SSE. `EXTERNAL` Claude Code ≥2.0.65 auto-sends `anthropic-beta: oauth-2025-04-20` ([claude-code#13770](https://github.com/anthropics/claude-code/issues/13770)); LiteLLM's oat-passthrough rewrites it ([litellm#22398](https://github.com/BerriAI/litellm/issues/22398)). `PARTIAL`: whether our chain must inject/forward it — spike Ph0.
- `TokenSource` reads the token **once at startup with a timeout, caches in memory** for process lifetime; keychain is never touched on the request path (`AUDIT` finding 8 — closes the blocking-`security`-call root #1/#5 on reads as well as writes).
- The token is never logged (redaction test §8.4).
- `AUDIT-IMPL 2026-06-12` **Q1 RESOLVED by Ph0 spike — PASS, no STOP.** Live chain (container claude-cli 2.1.175 → headroom 0.24.0 `:8787` → host proxy `127.0.0.1:8788` Bearer-swap → api.anthropic.com) with a real 108-char `sk-ant-oat01` subscription token: text reply and stream-json both 200/success, in BOTH matrix variants (proxy injecting `anthropic-beta: oauth-2025-04-20` and not injecting). Containerized Claude Code did NOT emit the oauth beta itself (client-beta observed: claude-code-20250219, context-1m-2025-08-07, interleaved-thinking-2025-05-14, context-management-2025-06-27, prompt-caching-scope-2026-01-05, mid-conversation-system-2026-04-07, effort-2025-11-24[, structured-outputs-2025-12-15]) — the API accepts the oat Bearer regardless, so the production authproxy needs NO beta-header manipulation. Client cancels a parallel request at CLI exit (one 502 "context canceled" after success) — normal, must not be treated as degradation. The pre-spike risk record (#28091, Feb-2026 ToS) stays below for history; the genuine-client+header-swap case is confirmed working as of 2026-06-12. `HISTORICAL`: the spike code is deleted (by design, Ph0 row below) and this evidence is not re-runnable; it was produced on claude-cli 2.1.175 (npm latest) while the container ships the newest signed apt-stable pin (2.1.153-1 as of 2026-06-12) — the shipped-version chain is exercised by every real launch (first-launch e2e), not by this spike record.

### 4.7 Container (D15, D16)

- Dockerfile: + claude CLI and gh, **pinned**. `EXTERNAL` install vectors — prefer Anthropic's signed apt repo (`downloads.claude.ai/claude-code/apt`, package `claude-code`) over npm for a Debian image (npm package is a native-binary wrapper using optional deps — [setup docs](https://code.claude.com/docs/en/setup)); gh from the official apt repo ([cli install_linux](https://github.com/cli/cli/blob/trunk/docs/install_linux.md)). Versions from `devcontainer-lock.json` (`ANCHOR .devcontainer/devcontainer.json:8-12`). `docker-ce-cli` is baked **unconditionally** (inert without a socket) so the D16 flag has a client to use (`AUDIT` finding 4).
- compose: feature lifecycle hooks replaced — host runs `provision --phase create/start` after `up` as pipeline steps (visible in cmdlog).
- docker.sock: enabled via a **compose override file** `compose.sock.yml` (`docker compose -f docker-compose.yml -f compose.sock.yml`), **not** a profile — profiles toggle whole services and cannot add a volume to the always-on `mirabilis` service, and a second service would collide on `container_name` (`AUDIT` finding 4). Off by default (D16).
- `sandbox.fingerprint` = git-short + STACKS + sock-override-applied (replaces `IsStale`; token excluded — no longer in env). Mismatch ⇒ re-create (G7).
- Docker Go SDK: inspect/events (`EXTERNAL` [moby/moby/client](https://pkg.go.dev/github.com/moby/moby/client); shipped as `github.com/moby/moby/client v0.4.0` — the post-split module, superseding the draft's `docker/docker v28 +incompatible` pin). `compose up/build/down` via CLI — `AUDIT`: an official Compose Go SDK now exists ([compose-sdk](https://docs.docker.com/compose/compose-sdk/)), but we choose the CLI deliberately because its output streams into cmdlog (visibility, G5); this rationale replaces the draft's incorrect "no SDK exists".
- devcontainer CLI removed from bootstrap/Makefile/install.sh. `QUESTION Q4` (blocks only `.devcontainer/devcontainer.json`): delete or keep minimal for VS Code UX? Default `ASSUMPTION` delete (`vscode.go` already uses the `attached-container` URI — `ANCHOR internal/runtime/vscode.go:26-28`); final at PR review.

### 4.8 Host process lifecycle & memory restore (D11, G6)

```
mirabilis started ── obs sink up; goroutines: authproxy(boot), status watcher, notify watcher (if configured)
   └─ TUI loop ── launch pipeline ── app runs tea.Exec(docker exec … claude) ── back to menu
mirabilis exited ── all goroutines die with the process; no children (I5)
```

- `AUDIT` (finding 8): the authproxy goroutine starts **at boot** (§4.8), so there is **no** "proxy" pipeline step (it would be a dead Check); proxy state surfaces in the header via `StatusChanged`. This removes the draft's duplicated/contradictory proxy step.
- `AUDIT` (finding 7): `membackup` is **Save-only** (host `docker cp`, `ANCHOR internal/runtime/docker.go:89`). Restore stays a `provision-create` sub-step reading the bind-mounted `.mirabilis/saved-memory` (`ANCHOR internal/provision/provision.go:52`) — one restore path, no dual source (D12). §5 Reset states: "memory returns on next launch via provision-create restore".
- `AUDIT` (finding 13): single-instance guard — a flock on a runtime file at TUI start refuses a second instance (`container_name: mirabilis` is shared). Host death mid-pipeline is survivable by design (Checks converge, G7) — stated as a feature. `restart: unless-stopped` means a daemon-restarted container runs without start-phase provisioning until the next launch; harmless only because Checks are idempotent — also stated.
- `tg-outbox` subcommand disappears (`ANCHOR cmd/mirabilis/main.go:62-85`); the watcher is a goroutine.

### 4.9 Secrets, migration, and chat-id (D12, G8)

- One backend per platform: keychain (macOS) / file 0600 (Linux/WSL). Entry name `mirabilis-<key>` (the doubled `-token` of `ANCHOR runtime_darwin.go:24,46` is fixed).
- `AUDIT` (finding 5): one-time **migration** in the secrets adapter's `Get`-miss path — read the old doubled keychain name / old plaintext file → write the new entry → delete the old file and old keychain entry. ~15 lines, unit-tested (§8.1). Without this every existing install is silently re-prompted and old plaintext files are orphaned.
- `AUDIT` (finding 6): Telegram **chat-id** moves out of create-env (today `docker-compose.yml:30` → `internal/runtime/env.go:20`, stale forever like the Claude token) into the already-bind-mounted `.mirabilis/` dir — host writes a `chat-id` file next to the outbox; hooks read it per-event. Channel auto-detection (`ANCHOR internal/hooks/hooks.go:358-415`, currently needs the bot token *inside* the container — impossible under D12) moves **host-side** into `engine/notify`, which already holds the token.

---

## 5. Flows

**First launch**: menu → Launch → pipeline: preflight ✔ → claude-auth (Check: no token → app runs `tea.Exec claude setup-token`; the pty-interposer tees the child to the real terminal while scanning scrollback for the token; on exit `Store.Set` in background) → stacks/plugins/skills forms (Check: no persisted choice → show) → telegram (Check: not configured and not skipped → form; "Skip" persisted) → image build (streamed) → container up → provision create/start (incl. memory restore, headroom install + upstream=host-proxy) → gh-auth (device flow, code+URL) → plugins/skills/harness → app runs `tea.Exec claude` → exit → menu.

**Repeat launch**: all Checks satisfied → pipeline flies green → straight to claude (I9, G7).

**Adapter swap** (G8): add `internal/engine/notify/slack.go` + one registration line. Nothing else.

**Reset**: confirm → `membackup.Save` (if preserve) → `sandbox.Reset` → menu with notice; memory returns on next launch via provision-create restore.

---

## 6. Anchor map: old node → fate (file/function granularity for multi-responsibility packages)

`AUDIT` (cross-cutting): `internal/hooks` and `internal/provision` each hold several unrelated responsibilities, so they are mapped per file/function; package-level mapping hid headroom (finding 11).

| Current (`531a56b`) | Fate | Destination |
|---|---|---|
| `cmd/mirabilis/main.go` | rewritten; dispatch kept; `tg-outbox` → goroutine | `cmd/mirabilis` |
| `cmd/tgsend`, `cmd/tgsmoke` | folded into `notify send` / adapter tests | `engine/notify` |
| `internal/app/{app,menu,forms,run}.go` | **replaced** (§2 #1,2,7) | `tui/*` |
| `internal/pipeline/*` | replaced; deps/optional/retry carried over; streaming + Resume added | `engine/pipeline` |
| `internal/steps/*` | rewritten as `Command` with streaming; Check/Run logic carried over | `engine/steps` |
| `internal/runner` | Fake idea kept (lazygit `FakeCmdObjRunner`; note: not literally GUI-free upstream) | `engine/exec` |
| `internal/runtime/docker.go` | exec→SDK; `IsStale`→fingerprint; `SaveMemory`→membackup | `engine/sandbox`, `engine/membackup` |
| `internal/runtime/env.go` | dies: env assembly → sandbox; keychain reads → secrets; chat-id leaves create-env | — |
| `internal/runtime/handoff.go` | `syscall.Exec`→`tea.Exec` attach (§2 #9); exec env enumerated (§7, finding 12) | `engine/sandbox` + `tui/app` |
| `internal/runtime/runtime_darwin.go` | keychain → secrets adapter; names fixed + migration (§4.9); read once + timeout | `engine/secrets` |
| `internal/runtime/vscode.go` | carried over; VS Code limitation documented (finding 14) | `engine/sandbox` |
| `internal/provision/{telegram,claude}.go` | secrets → secrets/claudeauth; dual-write removed (D12) | `engine/secrets`, `engine/claudeauth` |
| `internal/provision/status.go` | sync snapshot → status (initial inspect + events resubscribe, off UI thread) | `engine/status` |
| `internal/provision/headroom.go` | `AUDIT` **kept** but re-purposed: headroom install + MCP register; proxy upstream set to host-proxy at provision-start (D27); no longer the terminal base-URL | `engine/steps/headroom` |
| `internal/provision/*` (rtk, settings, memory, hud, mcp, harness, plugins, skills, statefile, gitidentity) | in-container phases carried over, shaped as steps; `RestoreMemory` stays provision-create sub-step | `engine/steps` |
| `internal/ghauth` | carried over (streaming correct); special Msg → generic `NeedInteractive`/`Resume` | `engine/steps/ghauth` + screen |
| `internal/telegram`, `internal/tgtoken` | behind the Notifier port; tgtoken dies into secrets | `engine/notify` |
| `internal/hooks` — notify half | writes to the notify outbox; reads chat-id from bind-mount (§4.9) | `engine/notify`/hooks |
| `internal/hooks` — `ensureProxyForSession` (`:317-340`) | `AUDIT` **changed**: stops re-writing `ANTHROPIC_BASE_URL`; only ensures headroom is alive with upstream=host-proxy (D27); idempotent | hooks |
| `internal/ui` | strings EN-only (D18), styles | `tui/strings`, `tui/styles` |
| `internal/config` | carried over; + tunables (G4) | `engine/config` |
| `.devcontainer/*` | features → Dockerfile; json — Q4 | minimal or deleted |
| `docker-compose.yml` | + `compose.sock.yml` override; secret/chat-id create-env removed | same file + override |
| `Makefile`, `install.sh`, `Brewfile` | − devcontainer CLI; + claude CLI (D17); `up`=compose | same files |
| `.golangci.yml` | `AUDIT` F9: + forbidigo rules enforcing I2/I4/I13 (§8.6); currently only govet/staticcheck/unused/gofmt | same file |
| `AGENTS.md`, `SECURITY.md`, `README.md` | revision §11 | same files |
| `test/*.bats`, golden | kept and extended | `test/`, `tui/testdata` |

Completeness: every package and every multi-responsibility function above is accounted for; no orphan nodes (I11). New nodes vs draft: `obs` (G5), `compose.sock.yml`.

---

## 7. Pipeline step registry (target)

| Step | Check (idempotency) | Run | Kind |
|---|---|---|---|
| preflight | docker reachable, compose file valid | start Docker Desktop (darwin) / hint | Auto |
| claude-auth | `TokenSource.Token()` present | emit `NeedsTerminal{claude setup-token}`; app pty-interposes, parses, `Store.Set` bg | Terminal |
| stacks/plugins/skills | choice persisted | `NeedInteractive{options}`; multiselect form; write config | Interactive |
| telegram | configured **or** explicitly skipped | `NeedInteractive`; select+token form; `Store.Set` bg w/ timeout | Interactive, optional |
| image | fingerprint matches | `docker compose build` (streamed) | Auto |
| container | running && fingerprint ok | `compose up -d`, wait healthy | Auto |
| provision-create/start | statefile in container (incl. memory restore, headroom upstream set) | `docker exec mirabilis provision --phase …` | Auto |
| gh-auth | `gh auth status` ok in container | device flow with streaming | Interactive |
| plugins / skills / harness | container state matches choice | install/remove | Auto, optional |
| attach | — (terminal) | emit `NeedsTerminal{docker exec -e TERM -e COLORTERM -e GITHUB_PERSONAL_ACCESS_TOKEN … claude --append-system-prompt-file …}` (`ANCHOR handoff.go:54-79`; GH token + TERM/COLORTERM retained — finding 12. `AUDIT` F8: no `-e ANTHROPIC_BASE_URL` — the base URL lives in container `settings.json` via the chain, §4.6) | Terminal |

There is **no** "proxy" step (boot-time goroutine — §4.8).

## 8. Tests (D8, G7) — full pyramid

1. **Engine unit** — every node against `exec.Fake` (lazygit ExpectArgs); secrets file backend + migration; bus routing (address, broadcast, stop, Resume incl. Cancelled).
2. **Idempotency contract** (G7) — table over the registry: `Run(fake)` ⇒ `Check==true`. `AUDIT` (finding 9): interactive steps run under a **fake interactive resolver** (canned `Resume` per step); `Kind: Terminal` steps (Check "—") are exempt. A step failing the contract cannot be registered.
3. **Snapshot/golden TUI** — teatest v2 (`EXTERNAL` module path `github.com/charmbracelet/x/exp/teatest/v2`, experimental pseudo-versions — pin explicitly): menu, launch progress, cmdlog, forms; successor of `TestFlowMenuGolden.golden`. One golden asserts launch-frame latency under a slow fake runner (empirically catches blocking in Update — finding 15).
4. **authproxy** — httptest upstream: Authorization injection, refusal without session key, token absent from logs, SSE pass-through, `anthropic-beta` handling per Ph0 outcome.
5. **e2e + user-scenario** — clean machine → install.sh → mirabilis → full first launch → exit claude → menu → repeat launch < 10 s → reset; plus business scenarios: "token rotation without re-create", "Telegram configured after container up delivers a notification", "messenger adapter swap", "node failure → degraded, menu still works" (G6).
6. **Smoke/CI** — `test/install.bats`, `pre_commit.bats` kept; + bats `mirabilis --help`/`provision --phase` dispatch. Matrix ubuntu+macos: `go test ./...`, golangci-lint (forbidigo: `os/exec` only in `engine/exec`; `os.ReadFile|os.WriteFile|net/http` forbidden in `tui/**` except `tui/app` Cmd constructors — finding 15), bats. Keychain tests skipped in CI (`ANCHOR runtime_darwin.go:32`). `claude -p` spike note: from 2026-06-15 subscription `-p` draws a separate monthly Agent SDK credit (`EXTERNAL` auth docs) — budget the spike accordingly.

## 9. Implementation phases (one PR, sequential commits)

| Phase | Content | Definition of Done |
|---|---|---|
| **0. Auth-proxy spike** | minimal chain under `test/spike` (deleted at PR end): oat token on host, headroom upstream→host proxy, container `claude -p "ping"`; test with/without `anthropic-beta: oauth-2025-04-20`; record whether containerized Claude Code emits it; Linux listen interface (Q3) | model reply + streaming work; `/status` shows auth ok; facts recorded here. **Failure → STOP + discuss incl. ToS (D19/D28)**; alternatives §15 |
| 1. Foundation | `bus`, `obs`, `engine/exec`(+fake), `engine/secrets`(+migration), `engine/config` | units green; entry names without doubles; migration test green; secret read/write with timeout |
| 2. Sandbox | SDK client, compose wrapper + override, fingerprint, events, `engine/status` (initial inspect + resubscribe) | up/detect tested with fake; events → snapshot; cold-start (docker down→preflight→resubscribe) tested |
| 3. Pipeline | `engine/pipeline` FSM + Resume + non-interactive steps | idempotency contract green for all registered-so-far steps |
| 4. Services | `authproxy`, `claudeauth`(pty-interposer), gh-auth, `notify`(telegram + host chat-id + watcher), `membackup`(Save), headroom re-purpose; close Q2 by reading `internal/hooks` | proxy tests green; Q1 result recorded; Q2 resolved as a fact here |
| 5. TUI | frame, router, components, screens; bidirectional bridge incl. `tea.Exec`+pty | goldens green; latency golden green; lint forbids I/O in tui (I2) |
| 6. Switch | `cmd/mirabilis` onto new nodes; Dockerfile/compose/override/Makefile/install.sh/Brewfile; **delete** old packages; AGENTS/SECURITY/README | no dead code; CI green |
| 7. Verification | e2e + user-scenarios, invariants §10 executed, owner sign-off | PR ready |

## 10. Invariants (sign-off table)

| ID | Invariant | Verification |
|---|---|---|
| I1 | Real Anthropic token never in the container: not env (create or exec), not fs, not image, **not `settings.json`** | e2e: `docker inspect` + exec env dump + `grep ANTHROPIC_BASE_URL`/token in container `settings.json` & image |
| I2 | UI thread does no I/O | forbidigo (`os/exec`, fs, `net/http`) in `tui/**` except `tui/app` Cmd ctors; + latency golden |
| I3 | Every non-terminal step idempotent: Run ⇒ Check=true | contract test §8.2 |
| I4 | Every command visible: spawn only via Runner; Runner always emits `started{Argv}` to cmdlog + obs log | forbidigo + bus unit |
| I5 | Host process leaves no children after exit | e2e: ps after quit |
| I6 | Port adapter swap = new file + registration; zero edits elsewhere | structure review; demo adapter test |
| I7 | A secret lives in exactly one backend per platform; old locations migrated then deleted | secrets unit + migration unit |
| I8 | All UI strings in `tui/strings`, English | grep test |
| I9 | Repeat launch on a healthy system: zero questions, zero changes | e2e checklist |
| I10 | docker.sock absent by default; enabling (override) changes fingerprint | fingerprint unit + e2e inspect mounts |
| I11 | §6 map covers every package and every multi-responsibility function; no dead code after switch | PR diff review |
| I12 | A single node's failure never blocks the menu; status shows `degraded` (G6) | unit (fault-injected adapter) + user-scenario test |
| I13 | One observability sink: every node logs via `obs`; nothing writes elsewhere (G5) | forbidigo on stray `log`/`fmt.Fprint(os.Stderr`; review |

## 11. Document edits

- **AGENTS.md**: Layout → new graph (engine/tui/obs); Boundaries → "open egress stays; api.anthropic.com via claude→headroom→host auth proxy; the real secret never enters the sandbox; sandbox provisions, harness behaves (G2)"; remove devcontainer CLI.
- **SECURITY.md**: threat model with the chained proxy and no default socket; the `compose.sock.yml` opt-in and its consequence; VS Code attach uses the session-keyed chain (finding 14).
- **README.md**: quick start (bootstrap installs claude CLI), single secret flow, new TUI screenshot.

## 12. Environment cleanup checklist (owner-executed, outside repo scope — D20)

1. `kill 89506` — orphaned tinyproxy (removed version; alive 3.5 days).
2. `kill 68036` — stuck `security add-generic-password` (holds the neighbouring terminal).
3. `security delete-generic-password -s mirabilis-telegram-token-token` (and `-claude-token-token`) — old doubled names (after the new scheme + migration ship).
4. Decide the fate of the repo checkout in `$HOME` (cmd/, internal/, go.mod at home root) — looks accidental.
5. Remove the stray tinyproxy log/conf under `$TMPDIR`.

## 13. Glossary

- **Node** — a package with one responsibility; the unit of replacement and bug localization (G3/G8).
- **Port / adapter** — a node's interface / its swappable implementation (D14/G8).
- **Bus** — the bus-message-type registry + delivery rules (upward via Cmd, downward via Envelope); the engine↔tui bridge is its only crossing.
- **Resume** — the inbound half of the bridge: how an interactive/terminal step continues (or is `Cancelled`).
- **Step (Command)** — an idempotent pipeline unit with Check/Run/Meta and output streaming (G7).
- **Fingerprint** — deterministic digest of the desired container config (git-short + STACKS + sock override); mismatch ⇒ re-create (G7).
- **Session key** — one-time auth-proxy access key; the only "secret" the container holds, useless without the live host proxy.
- **Chain** — claude → headroom (observability/MCP) → host auth proxy (token injection) → Anthropic (D27).
- **obs** — the single observability sink: slog file + node-status registry feeding the header and cmdlog (G5).
- **cmdlog** — the panel of actually executed commands and their output (anti-black-box).

## 14. Open QUESTIONs

| ID | Question | Blocks | Resolution |
|---|---|---|---|
| Q1 | Subscription oat token through the chained Bearer proxy: accepted mid-2026? beta header needed? streaming stable? ToS posture? | authproxy, claudeauth | `AUDIT-IMPL 2026-06-12` RESOLVED at Ph0: accepted (200 + stream-json success, both with and without proxy-injected `oauth-2025-04-20`); claude-cli 2.1.175 does not emit the oauth beta itself; no header manipulation needed; details in §4.6 |
| Q2 | How hooks use tgsend / the container→host queue contract | notify, hooks | `AUDIT-IMPL 2026-06-12` RESOLVED: queue dir `<repo>/.mirabilis/outbox` (`MIRABILIS_REPO` env, default `/workspace`); hooks (`hooks.go:75-107`) and tgsend write atomic `<id>.job` JSON {id, chat_id, text, created_at} via tmp+rename (`telegram/queue.go`); host watcher takes `.job` without matching `.status`, one send attempt, writes `<id>.status` {id, ok, error, completed_at} — at-most-once on failure; no token in files; chat-id today from container env `TELEGRAM_CHAT_ID` (the §4.9 staleness bug) — moves to the `.mirabilis/chat-id` file per finding 6 |
| Q3 | Host-proxy listen interface on Linux (loopback vs bridge) | authproxy (linux) | `AUDIT-IMPL 2026-06-12` RESOLVED: darwin — loopback `127.0.0.1` verified live at Ph0 (Docker Desktop tunnels `host.docker.internal` to host loopback); linux — `host-gateway` maps to the bridge gateway IP (documented Docker semantics), a loopback-bound listener is unreachable from containers, so authproxy binds `0.0.0.0` on linux, gated by the per-session key (the swap design keeps the real token unexposed); linux live check folds into CI/e2e |
| Q4 | `.devcontainer/devcontainer.json`: delete or keep minimal | that file only | PR review; default delete |
| Q5 | headroom-ai upstream/base-URL config key for chaining to the host proxy | headroom step, authproxy | `AUDIT-IMPL 2026-06-12` RESOLVED: env `ANTHROPIC_TARGET_API_URL` / `--anthropic-api-url` (headroom-ai 0.24.0); client Authorization forwarded upstream (only `x-headroom-*` stripped) |

## 15. Auth alternatives (logged per D28; used only if Q1 fails at STOP)

Ordered by fidelity to D6 ("token stays on host"):

1. **Host proxy, primary (D6/D27).** Token on host; container holds only a session key. Risk: Q1.
2. **apiKeyHelper broker.** In-container `apiKeyHelper` fetches a short-lived token from a host broker (unix-socket/`host.docker.internal`) on a TTL/401 (`EXTERNAL` auth docs: rank 4, default 5-min/401 refresh). Keeps the long-lived secret on host; **but** billing becomes API-key/pay-per-token, not the Max/Pro subscription — a product change requiring owner sign-off.
3. **Env injection of `CLAUDE_CODE_OAUTH_TOKEN`** at create/restart (`EXTERNAL` rank 5, headless path). Guaranteed to work with the subscription, but the real token enters the container — **violates D6**; last resort, owner decision only.

Each carries its own invariant impact on I1; switching among them is one adapter swap in `engine/claudeauth` (G8), so the decision can be deferred to the STOP without architectural cost.
