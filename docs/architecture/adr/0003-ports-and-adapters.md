---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# Ports are the only use-case↔adapter coupling; adapters are replaceable

## Context and Problem Statement

mirabilis integrates with several external surfaces (subprocess execution, the OS keychain, the Docker daemon, Telegram, a host-local model, the Anthropic API). These must be swappable and fakeable in tests without editing the logic that uses them (G8: swapping an adapter is one new file plus one registration line, zero edits elsewhere).

## Decision Drivers

- G8 replaceable nodes; I6 (adapter swap = new file + registration).
- Testability: every external surface needs a fake.
- G6 graceful degradation: a failed adapter degrades its own function only.

## Considered Options

- Consumer-defined narrow port interfaces, one per external surface, implemented by adapters.
- A single shared god-interface aggregating all capabilities.
- Concrete types passed directly (no interface seam).

## Decision Outcome

Chosen: **consumer-defined narrow ports**. The use-cases depend on the port interface; each adapter implements exactly one port; selection happens at the composition root (`cmd`). The ports are: `Runner`, `Store`, `Docker`, `Notifier`, `Completer`, `TokenSource` (6). `obs` is intentionally **not** a port — it is the one concrete sink injected everywhere (see ADR-0004); promoting it to a port is a possible future improvement, recorded here as out of scope, not as a defect.

### Consequences

- Good: adapters are replaceable and fakeable (`exec.Fake`, `sandbox.FakeDocker`); a new platform backend is one file + one registration; degradation is local.
- Bad: interface proliferation if ports are split too finely — kept in check by "one port per external surface".
- Neutral: a few adapters (`status`, `membackup`, `localllm`) are driven directly by the composition root rather than through a use-case; that is honest facade wiring, not a port violation.

### Confirmation

Compile-time guards `var _ Port = (*Adapter)(nil)` for each adapter; a demo-adapter test exercises the swap; I6 review on every PR.
