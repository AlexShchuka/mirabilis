---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# One observability sink: logs and status converge on `obs`

## Context and Problem Statement

A node's failure must be visible, and the TUI status header and the log file must reflect the same truth (G5: every node's logs and status converge on one destination; nothing writes into the void).

## Decision Drivers

- G5 single sink; G6 degraded visibility (a node shows `degraded`, the menu keeps working).
- I13: one observability sink, mechanically enforced.

## Considered Options

- A single `obs` sink (slog file + thread-safe status registry) injected everywhere.
- Per-package loggers writing to their own destinations.
- Scattered `os.Stderr` / `log.*` writes.

## Decision Outcome

Chosen: **a single `obs` sink** — a concrete struct injected into every component that logs or reports health. Each component logs via `obs.Logger(node)` and reports health via `obs.Set(node, state, detail)`; the status snapshot fans out to the TUI header from the same stream. Three locations are exempt because they run as separate short-lived processes with no reachable in-process sink: `internal/obs` itself, `cmd/mirabilis`, and `internal/hooks` may write `os.Stderr`.

### Consequences

- Good: one place to read state; the status header and the log are never out of sync.
- Bad: `obs` is a common dependency reachable by all components — accepted as the one sanctioned shared node; it is concrete (no interface), so replacing the sink touches every signature (a known cost, not a planned change).
- Neutral: the exemptions are explicit and enumerated, not implicit.

### Confirmation

forbidigo `^os\.Stderr$` rule with the documented exclusions (`internal/obs`, `cmd/mirabilis`, `internal/hooks`, `_test.go`); I13.
