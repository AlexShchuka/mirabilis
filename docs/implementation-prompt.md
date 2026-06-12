# Implementation prompt — mirabilis greenfield (one PR)

You are Claude Code, implementing the mirabilis greenfield rewrite **in a single coherent PR**. This file is your entry point and your operating contract. Read it fully, then execute. Work autonomously; halt only at the defined STOP gate or a genuine owner-only decision.

## 1. Read these first, in order (they are the source of truth — this prompt does not restate them)

1. `docs/redesign-plan.md` — **the authority.** Architecture, node graph (§4), dependency edges (§4.2), bus & bridge (§4.3), step engine (§4.5), auth chain (§4.6), container (§4.7), lifecycle (§4.8), secrets/migration (§4.9), flows (§5), anchor map (§6), step registry (§7), tests (§8), phases (§9), invariants (§10), doc edits (§11), glossary (§13), open QUESTIONs (§14), auth alternatives (§15). Meta-goals G0–G8 (§0) and decisions D1–D28 (§3) outrank everything else; on conflict G0 wins (system over nodes).
2. `docs/session-log.md` — the owner's verbatim directives and the audit trail (why each decision exists). Consult when a decision's intent is unclear.
3. `docs/plan-draft.md` — pre-audit draft, kept only as history. Do not implement from it; where it disagrees with the final plan, the final wins.
4. `AGENTS.md` / `SECURITY.md` — repo rules (no comments in code; no push to main; one coherent landing; Uber Go style; tests required). These bind you.

The plan was vetted by four independent audits (code-anchors 15/15, web-facts, two adversarial). It is **READY-TO-IMPLEMENT**. Do not relitigate D1–D28 or G0–G8. You may refine *how*, never *what*.

## 2. Mission

Replace the current TUI + provisioning + container stack with the architecture in `redesign-plan.md`, plus its full test pyramid, deleting the old packages in the same PR (§9 phase 6). End state: `go test ./...` green, `golangci-lint` green (with the new forbidigo rules, §8.6/F9), bats smoke green, and every invariant I1–I13 (§10) satisfied with executed evidence — not claims.

## 3. Non-negotiables (violating any of these means the work is wrong)

- **No comments in code or config** (D10/AGENTS.md). The graph carries meaning through node boundaries and precise names. Prose lives in `.md` only.
- **Engine has zero bubbletea/bus/tui imports** (D2/§4.2). `os/exec` only in `engine/exec`. Enforce mechanically (forbidigo) — do not rely on review alone.
- **UI thread does no I/O** (I2). All I/O via `tea.Cmd`; interactive/terminal steps emit `NeedInteractive`/`NeedsTerminal` and `tui/app` runs `tea.Exec`, never the engine.
- **Real Anthropic token never enters the container** (I1) — only the session key does. The chain is claude→headroom(:8787)→host proxy→Anthropic (D27/§4.6).
- **Every node is replaceable** (G8): a port + adapter; adding an adapter = one file + one registration line.
- **Idempotency contract** (G7/I3): every non-terminal step's `Run` ⇒ immediate `Check==true`. A step that fails the contract test is not registered.
- **Single observability sink** (G5/I13): all logging via `obs`; nothing writes elsewhere.
- **Branch, never push to main; one PR, no churn** (AGENTS.md). Minimal PR description.
- **Code is truth** (AGENTS.md #1): back every "done" with executed tool output. "Not green" = not done.

## 4. The STOP gate (do not cross without the human)

**Phase 0 is an auth-chain spike and a hard gate (D19/D28/§9).** Build the minimal chain under `test/spike`: host holds the oat token, headroom upstream → host proxy, container runs `claude -p "ping"`. Test with and without the proxy injecting `anthropic-beta: oauth-2025-04-20`; record whether containerized Claude Code emits it itself; resolve Q3 (Linux listen interface) and Q5 (headroom upstream config key).

- **If the spike succeeds:** record the resolved facts inline in `redesign-plan.md` (flip the relevant `PARTIAL`/`QUESTION`), then proceed to phase 1.
- **If the spike fails** (subscription oat rejected through the Bearer chain — the documented mid-2026 risk, Q1): **STOP. Do not pick an alternative yourself.** Surface to the human: the spike result, the technical reason, and the ToS dimension, then present §15's logged alternatives (host-proxy retry variants → apiKeyHelper broker → env injection, the last violating D6). The alternative choice is the owner's; it is one adapter swap in `engine/claudeauth` (G8), so nothing downstream blocks on it.

The spike needs Docker, the host `claude` CLI, and a real subscription token — these require the human's machine and authorization. Request them; do not assume them.

## 5. Execution loop (per phase, §9 phases 0–7)

Run phases sequentially (one PR, sequential commits). For each phase:

1. **Implement** the phase's nodes per `redesign-plan.md`. Honor the dependency edges (§4.2) and the anchor map (§6) — carry over working logic from the cited `ANCHOR` locations rather than reinventing it (G1: dense, no slop).
2. **Test alongside** (D8, full pyramid §8): unit (against `exec.Fake`), idempotency-contract table (§8.2, fake interactive resolver for interactive steps), snapshot/golden (teatest v2 — pin `github.com/charmbracelet/x/exp/teatest/v2`), and the relevant slice of e2e/user-scenario (§8.5). A phase is not done until its DoD (§9) is met with green output.
3. **Fact-check** every `ANCHOR` you rely on against the actual code, and every `EXTERNAL`/`PARTIAL` you resolve against live official docs (Anthropic auth, Bubble Tea v2, Docker SDK/compose). Do not trust the plan's citations blindly — re-verify. If a fact is refuted, treat it as §6 below.
4. **Adversarially review** the phase's diff with a subagent (correctness + the invariants the phase touches + SRP/KISS/G1 — no dangling leaves, no code-for-code's-sake). Fix what it confirms.
5. **Commit** the phase with a terse message; move on.

Use subagents to parallelize independent work (e.g. several engine nodes in phase 1/4, or implement-vs-review). Spawn them with non-overlapping scopes. When a subagent claims success, verify with real test output before believing it (Code is truth).

## 6. When something is wrong or unclear (paper-to-code discipline)

- **Load-bearing contradiction or refuted fact** (changes architecture or a Dn/Gn): **STOP and ask the owner** (D23). Do not invent a resolution.
- **Minor discrepancy** (line numbers, wording, a non-structural fact): fix it, and note the fix inline in `redesign-plan.md` with an `AUDIT`-style tag so the trail stays honest.
- **A genuine gap the plan didn't foresee:** raise it as a `QUESTION` to the owner before coding that node — never fill the gap with a plausible default (this is exactly the neuroslop the project forbids).
- Open QUESTIONs Q2 (hooks/tgsend contract) and Q4 (`.devcontainer` delete vs keep) resolve by reading the code at their phase and recording the fact; Q1/Q3/Q5 resolve at the Ph0 spike.

## 7. Definition of done (the whole PR)

- All §9 phases complete; old packages (§6) deleted; no dead code (I11).
- `go test ./...`, `golangci-lint run` (incl. forbidigo I2/I4/I13), and bats smoke all green — paste the real output.
- Invariants I1–I13 (§10) each verified with executed evidence (the §10 table names each check).
- e2e + business-scenario tests pass (§8.5): full first launch, exit-to-menu, repeat-launch <10 s zero-change, reset, token rotation without re-create, Telegram-after-up delivers, adapter swap, node-failure-degraded.
- Docs updated (§11): AGENTS.md, SECURITY.md, README.md.
- PR opened against a feature branch (not main), minimal description (what + why), CI green.

Lead your final report with the BLUF: what shipped, test/lint/e2e status with output, which QUESTIONs were resolved and how, and any STOP you hit. Keep prose tight (the owner is a strong system thinker — facts over narration).
