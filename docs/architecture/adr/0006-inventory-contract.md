---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# Inventory configured-sentinel is the host→container provisioning contract

## Context and Problem Statement

The host resolves the owner's selection (skills, plugins) and the container reconciles the sandbox against it. The contract must distinguish three states — *configured-and-empty*, *not-configured*, and *unset/absent* — because conflating "nothing selected" with "nothing to do, healthy" is the empty=healthy mask that lets a wanted install be silently skipped.

## Decision Drivers

- Explicit third state, not a nil-vs-empty pointer trick (D2).
- One-shot cutover, no dual-read window (D4); rollback = `git revert`.
- G7 idempotent, level-triggered reconcile; I1 names only, never secrets.

## Considered Options

- **A configured-sentinel struct** `{version:int, configured:bool, skills:[], plugins:[]}` in a dedicated leaf package, serialized to `inventory.json` (0644).
- The implicit nil-vs-empty pointer trick.
- A k8s-style nilable `*[]T` for unset≠empty.

## Decision Outcome

Chosen: **the configured-sentinel struct** in the target package `internal/engine/inventory` (not yet on disk). The host writes `inventory.json` (names only, 0644, host-first mkdir); the container reads and reconciles level-triggered each launch; `configured=false ∨ absent ⇒ NotConfigured ⇒ explicit log`, never silent-healthy. `version` is a fail-fast guard (a reader on `version > known` fails loud), not a migration mechanism. In-repo precedent: `plugins.Plan.Configured`.

### Consequences

- Good: removes the empty=healthy mask; carries no secret (I1).
- Bad: D4 removes the compatibility window, so the cutover is hard (safe via PR atomicity + green gates + `git revert`).
- Neutral: the implicit `neuro-matrix@neuro-matrix` plugin stays harness-governed, not stored in inventory.

### Confirmation

reconcile unit: `configured=⊥ ⇒ NotConfigured` (not healthy), `configured=true,[] ⇒ install 0`; JSON round-trip; absent ⇒ default + log; `version > known` ⇒ fail loud.
