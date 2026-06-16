---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# The pipeline FSM with idempotent Check→Run→Check is the domain core

## Context and Problem Statement

Provisioning is an ordered set of steps with dependencies, optionality, retries, timeouts, an interactive wait/resume, and the requirement that a repeat launch on a healthy system changes nothing (G7 idempotency; I9 zero-question relaunch). The control flow must live in one place and be testable, with the use-cases only *composing* steps, not re-implementing orchestration.

## Decision Drivers

- G7 idempotency everywhere; I3 every non-terminal step idempotent (Run ⇒ Check=true).
- G6 every step has a timeout; optional steps cascade-skip; retry is bounded.
- Determinism and testability of the orchestration.

## Considered Options

- A pipeline FSM with a `Command{Meta, Check, Run}` contract as the domain core.
- Imperative provisioning scripts.
- An external/third-party task runner.

## Decision Outcome

Chosen: **the pipeline FSM**. `Command` is the contract; the executor owns dependency ordering, parallel auto-batches, retry, per-step timeout, streaming events, and Resume/cancel. Every non-terminal step satisfies Check→Run→Check. Use-cases (`steps`, `provision`) compose `[]Command`; they never re-implement control flow. The target `provModel` package holds the step contracts as pure domain so both use-cases realize one desired-state model.

### Consequences

- Good: repeatable launches (I9), one tested contract (`pipeline.Contract`), control flow not duplicated across planes.
- Bad: orchestration complexity concentrates in one package — inherent to an FSM, not a smell.
- Neutral: terminal/handoff steps are explicitly exempt from the idempotency contract.

### Confirmation

`pipeline.Contract` test asserts post-Run `Check == true`; I3; the I9 relaunch e2e checklist.
