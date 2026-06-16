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

Chosen: **the configured-sentinel struct** in the target package `internal/engine/inventory` (a target package: its contract is fixed here, its code is the convergence target — README §5/§7). The host writes `inventory.json` (names only, 0644, host-first mkdir); the container reads and reconciles level-triggered each launch; `configured=false ∨ absent ⇒ NotConfigured ⇒ explicit log`, never silent-healthy. `version` is a fail-fast guard (a reader on `version > known` fails loud), not a migration mechanism. In-repo precedent: `plugins.Plan.Configured`.

**Wire contract.** The serialized form is the seam; field semantics live in prose (no struct comments, D10):

```go
package inventory

type Inventory struct {
	Version    int      `json:"version"`
	Configured bool     `json:"configured"`
	Skills     []string `json:"skills"`
	Plugins    []string `json:"plugins"`
}
```

- `version` — fail-fast guard; a reader on `version > known` fails loud. D4 forbids a dual-read window, so no fallback format exists.
- `configured` — `false ∨ absent ⇒ NotConfigured`; `configured=true` with `skills:[]`/`plugins:[]` ⇒ *explicit none*. This is the third state that removes the empty=healthy mask.
- `skills` — selected group-names ∈ catalog (opt-in).
- `plugins` — enabled plugin-names ∈ catalog (opt-in); the host normalizes the legacy opt-out source as `pluginCatalog − PLUGINS_DISABLED`. The implicit `neuro-matrix@neuro-matrix` stays harness-governed, not stored in inventory.

**Resolution runs host-side.** The container's config root is the image-baked `/opt/mirabilis/config`, which is *not* the `./.mirabilis` bind-mount; the host's working tree is the live source of templates and catalogs. So `resolve : template → inventory` runs host-side (the host reads its live config), and the container reads *only* `inventory.json` — never the catalog. This is what lets template/catalog edits take effect without rebuilding the image.

### Consequences

- Good: removes the empty=healthy mask; carries no secret (I1).
- Bad: D4 removes the compatibility window, so the cutover is hard (safe via PR atomicity + green gates + `git revert`).
- Neutral: the implicit `neuro-matrix@neuro-matrix` plugin stays harness-governed, not stored in inventory.

### Confirmation

reconcile unit: `configured=⊥ ⇒ NotConfigured` (not healthy), `configured=true,[] ⇒ install 0`; JSON round-trip; absent ⇒ default + log; `version > known` ⇒ fail loud.
