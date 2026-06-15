# Plan — UX fixes + token-organ + architecture decouple

Owner-approved plan. Anchors are pointers (code-is-truth: the developer re-confirms each `file:line` before editing). Gates for every landing: `go test -race ./...`, `golangci-lint run ./...`, `bats`. No comments in code (D10). Minimal diff / YAGNI. No push to `main`; branch, do not push or open a PR unless the owner asks.

## Decisions (locked with owner)

| Bug | Decision |
|-----|----------|
| 2 — telegram skip never re-asks | **A**: three-state automaton; `skip` is NON-terminal → re-asks next launch. Only `configured` (token+chat-id) is terminal. |
| 3a — provision hint text jumps | **C**: move the active step's detail out of the tree into ONE fixed, left-aligned status line below the steplist. |
| 3b — preloader not spinning | Fix the **header spinner** (confirmed dead). Steplist spinner is code-sound — verify env, optional router broadcast insurance. |
| 4 — header + logo | BorderBottom separator + animated logo (small shimmer glyph + large animated welcome mark). |
| 1+5 — secondary lockdown | **Architecture C**: decouple proxy/notify into a detached `mirabilis serve`; every terminal = equal client; remove owner/secondary/flock-role/promote/applySecondary. |
| caveman | Activate via the plugin catalog (marketplace plugin), not the dead skills git-clone path. |
| headroom | Add a G4 config knob `HEADROOM_MODE` (default `cache` = zero behaviour change); do NOT switch to `token` by default. |

## Sequencing

- **Phase 1** (independent, low/medium risk, shippable): bug 2, 3a, 3b, 4+logo, caveman, headroom-knob.
- **Phase 2** (structural refactor): architecture C (bugs 1+5) + fix `attachStep.Check` (I3). Land AFTER Phase 1 is green and reviewed — keeps each landing coherent (repo principle #6, no churn).

---

## PHASE 1

### P1.1 — Bug 2: telegram three-state (decision A)

- **Symptom**: skip at first launch → never asked again.
- **Root (FACT)**: `internal/engine/steps/telegram.go:38-41` — `Check()` returns `true` when `.env` has `TELEGRAM=skip`; `:59-60` — `Run()` writes `TELEGRAM=skip`. Pipeline then skips the step forever. `internal/engine/steps/telegram_test.go:53-66` (`TestTelegramRunSkipPersists`) pins the OLD behaviour.
- **Change**: model three states — `unset` / `skipped` (non-terminal) / `configured` (terminal = token+chat-id present). `Check()` returns `true` ONLY for `configured`. `skip` must NOT make `Check()` true on the next launch (skip = "not now", re-ask). Decide skip's write: simplest is `Run()` on skip writes nothing terminal, so next-launch `Check()` is false. Keep `TelegramConfigured()`/`MarkTelegramConfigured()` (`cmd/mirabilis/facade.go:184-190`) as the menu-side terminal marker; ensure the pipeline path and the menu path agree on what "configured" means.
- **I9 reconciliation**: I9 ("repeat launch = zero questions") applies to a *configured/healthy* system; an un-configured telegram is not healthy-silent, so re-asking is correct, not an I9 violation (owner's framing: skip is non-terminal).
- **Files**: `internal/engine/steps/telegram.go`, `internal/engine/steps/telegram_test.go`.
- **Tests**: invert `TestTelegramRunSkipPersists` → assert `Check()==false` after skip; keep `configured`→`Check()==true`.
- **Risk**: low. Verify the I9 e2e checklist does not assert telegram-silence-after-skip.

### P1.2 — Bug 3a: provision detail jumps (decision C)

- **Symptom**: hint text shifts left/right between frames during provision.
- **Root (FACT)**: `internal/tui/components/steplist.go:229` renders `styles.Hint.Render(r.Detail)` with NO `Width()` — the detail column width = current stdout line length, so it floats. `Detail` is filled per row from `pipeline.EvLine` (`steplist.go:153`).
- **Change**: stop rendering per-row `Detail` inside the tree. Render only the ACTIVE step's last line in a single dedicated status line below the steplist: left-aligned, fixed width (`styles.Hint.Width(w).MaxWidth(w)` with `w = innerWidth`), truncated. Tree rows show title + glyph only.
- **Files**: `internal/tui/components/steplist.go` (apply/View), possibly `internal/tui/app/screens.go` (launch screen layout).
- **Tests**: steplist unit tests (active-line shown once, fixed position, left-aligned); golden if present.
- **Risk**: medium (layout change). Keep it inside `tui` (no engine import — depguard).

### P1.3 — Bug 3b: preloader animation

- **Symptom**: preloader does not spin during provision.
- **Root (FACT)**: `internal/tui/app/handlers_menu.go` `startLaunch()` never sets `a.busy=true` nor calls `a.startBusy()` (cf. `ActionHarness` path which does). So `frame.m.busy==""` and the header spinner branch (`internal/tui/frame/frame.go:212`) is never entered. The steplist spinner is a separate `spinner.Model`; `internal/tui/router/router.go:51` DOES forward `spinner.TickMsg` to the top screen and `steplist.Init` schedules the tick — it only goes static when `a11y.ReducedMotion()==true` (`internal/tui/a11y/a11y.go:14`, gated on `NO_COLOR`/`ACCESSIBLE`/`NO_ANIMATE`; `golden_test.go` sets `NO_COLOR=1`, which is why tests show it static).
- **Change**:
  1. In `startLaunch()` set `a.busy=true` + start the busy tick (mirror `ActionHarness`); clear in `handlePipelineDone` (`a.busy=false`, `a.frame.SetBusy("")`).
  2. (Insurance, optional) `router.go`: broadcast `spinner.TickMsg` to ALL stack screens instead of only top, so a launch screen under a pushed form keeps animating.
- **Files**: `internal/tui/app/handlers_menu.go`, `internal/tui/app/busy.go`, `internal/tui/app/handlers_pipeline.go`; optional `internal/tui/router/router.go`.
- **Tests**: app state test — busy true while pipeline runs, false on done.
- **Open verify**: confirm whether the owner's terminal sets `NO_COLOR`/`ACCESSIBLE`/`NO_ANIMATE` (if so, steplist spinner is intentionally static — reduced-motion, not a bug).
- **Risk**: low.

### P1.4 — Bug 4: header redesign + animated logo

- **Symptom**: header is one plain line, no separator; owner wants a bottom divider + a logo.
- **Root (FACT)**: `internal/tui/frame/frame.go:209-231` `headerView()` returns one plain line; `styles.Header` (`styles.go:27`) is bold+colour only; no border. `MainSize()` (`frame.go:176`) subtracts 2 (header+footer).
- **Change**:
  1. **Separator**: add `BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(colMuted)` to the header (`BorderBottom` adds +1 row — FACT lipgloss v2). Update `MainSize` `height-2`→`height-3` (`frame.go:176`).
  2. **Small logo glyph** (left of `mirabilis` title in the header): animated shimmer, 3 frames `⊕ → ⊛ → ※`, period ~700 ms. Static `⊕` when `a11y.ReducedMotion()`.
  3. **Large welcome mark** (on the welcome/menu screen, bpNormal only): animated, 2 frames, period ~700 ms, core `⊙`:
     - frame A (orthogonal streams):
       ```
          │
        ──⊙──
          │
       ```
     - frame B (diagonal streams):
       ```
        ╲   ╱
          ⊙
        ╱   ╲
       ```
     Static frame B when `a11y.ReducedMotion()`. Collapse to the small glyph (no large mark) at `bpNarrow`/`bpShort`.
  4. **Animation wiring**: add one slow chrome tick (~700 ms) at the app level driving a frame index; pass to header + welcome mark. Reuse the existing busy-tick pattern (`busy.go`). Tick is NOT scheduled when `ReducedMotion()` (no idle repaint storm in accessible mode). Period is a single named const (could later move to `config/`, G4 — not now).
- **Strings**: all glyphs/marks live in `internal/tui/strings` (I8, English package). No raw glyphs in frame/screens code.
- **Files**: `internal/tui/frame/frame.go`, `internal/tui/styles/styles.go`, `internal/tui/strings/strings.go`, `internal/tui/screens/menu.go`, `internal/tui/app/*` (tick wiring).
- **Tests**: `frame_test.go` — `TestResizeReflow` (24→`height-3`=21, 30→27), `TestMainAreaCropped`; `golden_test.go` `TestFrameMenuGolden` regenerate (`-update`). Add a test that ReducedMotion → static logo frame.
- **Open verify**: `⊕/⊛/※/⊙` are East-Asian-ambiguous width — confirm single-cell rendering in the owner's terminal (else header alignment shifts); golden locks the final form. If they render 2-wide, fall back to ASCII or pad accordingly.
- **Risk**: medium (golden churn + glyph width + always-on idle tick when motion enabled).

### P1.5 — Caveman activation

- **Symptom**: caveman (agent-out compression channel) is in the catalog but never active.
- **Root (FACT)**: `config/skills.txt:2` = `juliusbrussee/caveman`; `internal/engine/provision/skills.go` git-clones skills into `~/.claude/skills/` only if listed in `~/.mirabilis-skills` (absent) → step is a no-op. Canonical caveman is a Claude-Code **marketplace plugin** (`claude plugin marketplace add …` + `claude plugin install caveman@caveman --scope user`; the plugin manifest registers its own SessionStart/UserPromptSubmit hooks) — a git-clone gives raw files, not an active plugin.
- **Change**: move caveman from the skills catalog to the plugin catalog so the existing `pluginsStep` installs it the same way the harness plugin is installed. Concretely: add caveman's marketplace to the marketplaces catalog and `caveman@caveman` to the plugins catalog (confirm exact filenames — research referenced `config/plugins.txt` + `config.ReadMarketplaces`); remove the dead `config/skills.txt` entry.
- **Files**: `config/` plugins + marketplaces catalogs, `config/skills.txt`, `test/token_opt.bats` (update the assertion that checked skills.txt), possibly a small `cavemanStep` only if a dedicated marketplace add is needed (prefer reusing `pluginsStep`).
- **Open verify**: exact marketplace source/id from the official repo's `.claude-plugin/marketplace.json` (likely `claude plugin marketplace add JuliusBrussee/caveman`).
- **Tests**: `token_opt.bats` (caveman in the plugin catalog); provision plugins test.
- **Risk**: low-medium.

### P1.6 — Headroom mode knob (G4) [recommended]

- **Symptom**: compression off; mode not switchable without editing Go.
- **Root (FACT)**: `--mode cache` hardcoded at `internal/engine/provision/headroom.go:66` and `internal/hooks/session.go:75`; no `config.HeadroomMode()`. headroom itself reads `HEADROOM_MODE` (`proxy --help`: `Env: HEADROOM_MODE`). Live now: `cache`, 166 req/session, prefix-cache savings, compression 0.
- **Change**: add `config.HeadroomMode(repo)` (G4, default `"cache"`); thread it into both start sites (flag or `HEADROOM_MODE` env). Default keeps current behaviour exactly (lossless + prefix-cache). `token` is lossy (CCR rewrites frozen turns to a marker + `headroom_retrieve`) AND busts prefix-cache — modes are mutually exclusive; the knob just makes the trade-off owner-switchable.
- **Files**: `internal/engine/config/config.go`, `internal/engine/provision/headroom.go`, `internal/hooks/session.go`, `internal/engine/provision/headroom_test.go`, `test/token_opt.bats`.
- **Tests**: config default `cache`; start sites use the configured mode.
- **Risk**: low.

---

## PHASE 2 — Architecture C: decouple background from TUI (bugs 1+5)

- **Goal**: every terminal is an equal TUI client; the proxy+notify run as one detached background service. Remove the owner/secondary role, `flock`-for-role, `promoteLoop`, `PromotedMsg`, `applySecondary`, `promote`.
- **Root (FACT)**: `cmd/mirabilis/main.go:103-118` — one process runs BOTH `startSession` (proxy:8788 + notify, `main.go:122-133`) AND `tea.Run()`; secondary is gated by `acquireFlock` (`main.go:79-85`, `flock_unix.go:33`) and stays passive via `promoteLoop` (`promote_unix.go`). `internal/tui/app/app.go:89-93` `applySecondary()` is the sole source of the disabled menu actions (grep `SetEnabled` → only `applySecondary`/`promote`). Proxy port is a fixed singleton `defaultAuthProxyPort=8788` (`config.go:16`, `authproxy.go:68`); session key is a shared file (`cmd/mirabilis/sessionkey.go`); container reaches the proxy via `host.docker.internal:8788` (`facade.go:75`).
- **Design**:
  1. New `mirabilis serve` subcommand: runs proxy(8788)+notify, detached (`setsid nohup`), single-instance (binding 8788 is the natural guard — if already bound, exit 0 "already running").
  2. `mirabilis` (no-arg TUI): on start, ensure `serve` is running (spawn detached if not); connect as a pure client (read the shared session key); the TUI process does NOT run `startSession`.
  3. Lifecycle / I5 (host leaves no orphan): ref-count clients (e.g. a client pidfile/registry under `.mirabilis/`); when the last client exits, signal `serve` to stop. `serve` must self-terminate when client count reaches 0 (and on ctx cancel). This is the load-bearing detail — get the reap right or I5 regresses.
  4. Remove the role machinery: `applySecondary`/`promote` (`app.go`), the owner/secondary branch (`main.go`), `promoteLoop`/`PromotedMsg`. Keep `flock` only as the serve-singleton guard (not a UI role).
  5. Fix `attachStep.Check()` (`internal/engine/sandbox/attach.go:26` hardcodes `false`) → return true when the container/claude is already reachable (I3: Run ⇒ Check=true).
- **Files**: `cmd/mirabilis/main.go`, new `cmd/mirabilis/serve.go`, `cmd/mirabilis/facade.go`, `internal/tui/app/app.go`, `cmd/mirabilis/flock_unix.go` + `promote_unix.go` (repurpose/remove), `internal/engine/sandbox/attach.go`, + tests.
- **Risks** (owner accepted reliability cost for concurrency): spawn race when two terminals start simultaneously (mitigate via bind-or-skip on 8788); reap correctness (orphan proxy if reap fails → I5); security — proxy/token-injection lifetime now spans terminals while any client is up (acceptable per owner; document in SECURITY.md/AGENTS.md). Concurrent docker mutations from multiple clients are possible by design.
- **Tests**: serve single-instance (second `serve` is a no-op); client connects without starting a proxy; last-client-exit reaps serve (I5 e2e: `ps` after quit); attach Check idempotency; no owner/secondary references remain.

---

## Open verification (resolve during impl, code-is-truth)

- steplist spinner: owner-terminal `NO_COLOR`/`ACCESSIBLE`/`NO_ANIMATE`?
- logo glyph width (`⊕/⊛/※/⊙`) single-cell in owner terminal?
- caveman exact marketplace id (official `.claude-plugin/marketplace.json`).
- I9 e2e: no assertion of telegram-silence-after-skip.
- exact config filenames for plugins/marketplaces catalogs + how the harness plugin is currently registered (mirror it for caveman).

## Gate (every landing)

`go test -race ./...` + `golangci-lint run ./...` + `bats` all green; golden regenerated where noted; depguard (engine ↛ tui/bus; tui leaves ↛ engine) and forbidigo (UI no I/O, single obs sink) clean.
