---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# go-arch-lint is the reflexion model and fitness function; no comments in code

## Context and Problem Statement

A scheme that is not enforced decays. The target architecture must be expressed as an executable model so that completeness (every package mapped) and non-contradiction (no forbidden edge, no drift) are proven continuously, not asserted once in a document.

## Decision Drivers

- Completeness: an unmapped package is a literal gap and must fail the build.
- Non-contradiction over time: rules may only tighten (monotone), never loosen to pass.
- Prose lives in `.md`; code and config stay comment-free (D10).

## Considered Options

- `.go-arch-lint.yml` as the executable reflexion model + fitness function, gated in CI.
- A diagram-only architecture (Structurizr) with no runtime enforcement.
- Review-only enforcement (humans check conformance).

## Decision Outcome

Chosen: **`.go-arch-lint.yml` is the executable reflexion model and fitness function.** It maps every package to exactly one component and declares the allowed dependency edges; `go-arch-lint check` proves no *absence* (unmapped package) and no *divergence* (forbidden edge). The full fitness suite is `go-arch-lint check` + `golangci-lint run` (depguard, forbidigo, errcheck, unused) + `go test -race ./...` + `bats`. Rules are monotone: removing an allowed exception is permitted, adding a new violation is not; each tightening is an ADR. No comments in code or non-workflow config (D10); prose lives only in `.md`. The Structurizr `workspace.dsl` is a human-facing, deliberately curated view of the same model: as a C4 view it may omit edges for clarity, but it must never *contradict* `.go-arch-lint.yml` — drawing an edge the model forbids, or a description that denies an enforced rule, is a defect.

### Consequences

- Good: drift is caught in CI; the Structurizr/ADR scheme and the code cannot silently diverge.
- Bad: adding a package requires a component mapping — intentional friction that keeps the partition total.
- Neutral: the live `.go-arch-lint.yml` enforces the *current* 23 components; the two target packages (`inventory`, `provModel`) are added to it as the rewrite creates them — the live config is not loosened ahead of the code.

### Confirmation

CI green gate (`go-arch-lint check`, `golangci-lint run`, `go test -race`, `bats`); the `no-config-comments` CI job + pre-commit hook for D10.
