---
status: accepted
date: 2026-06-16
decision-makers: Owner
supersedes: 0001
---

# Six-role taxonomy: hexagonal core/use-case/driving/driven, refined with service and infrastructure

## Context and Problem Statement

ADR-0001 partitioned every package into four roles (domain-core / application-service / driven-adapter / driving-adapter). In practice that "driven" bucket conflated three different contract-kinds: packages that **implement a port** (`exec`, `secrets`, `sandbox`, `claudeauth`, `notify`, `localllm`), long-lived or one-shot **port-consumers started by the composition root** (`authproxy`, `status`, `membackup`), and **cross-cutting infrastructure behind no port** (`obs`, `config`, `harness`, `reconcile`). A node's role should tell a reader its contract-kind and its blast-radius; the four-role bucket did not.

## Decision Drivers

- A role must say *what a node's contract is* and confine change to one node (G0, G1, G3, G8).
- The classification must stay aligned with the inward dependency direction (the DAG is the proof, ADR-0001).
- Stay on named industry ground, not invent core-facing directions (Cockburn primary/secondary actors).

## Considered Options

- Keep four roles (the conflated "driven" bucket).
- Promote `service` and `infrastructure` to new top-level roles the core may depend on.
- Six roles where `service`/`infrastructure` are **named refinements of the two existing hexagon sides**, with the inward rule unchanged.

## Decision Outcome

Chosen: **six roles** — `core`, `use-case` (application service), `driving adapter`, `service`, `driven adapter`, `infrastructure`. The spine stays hexagonal (ADR-0001's axis is kept); `service` and `infrastructure` are **naming refinements of the two sides**, not new directions:

- **service** ⊂ the **driving / primary** side: a primary actor (Cockburn) started by the composition root that *consumes* a port and implements none — `authproxy` (reverse-proxy daemon), `status` (event-watcher), `membackup` (one-shot facade).
- **infrastructure** ⊂ the **driven / secondary** side: cross-cutting, injected, depended on by many, behind no port — `obs` (sink), `config` (the desired-state seam: reads/writes `.env` + env), `harness`, `reconcile`.
- **driven adapter**: implements exactly one of the six ports (`exec`, `secrets`, `sandbox`, `claudeauth`, `notify`, `localllm` — a port↔adapter bijection), or is a **port-less executor** driven by a use-case (`skills`, `plugins`).

The **inward dependency rule is unchanged**: the core depends on nothing; everything points inward; promoting `service`/`infrastructure` to roles the core may depend on would bend that rule and is the rejected anti-pattern. Each node is a black-box **contract** `{goal, input → output}`: the port *is* the contract (Parnas information-hiding, Meyer design-by-contract, Martin interface-segregation), and `go-arch-lint` is the reflexion model / fitness function that fails the build when an import crosses a seam the contract forbids — so "the contract is the only coupling" is mechanically enforced, and a change to one node is provably local.

### Consequences

- Good: a role now names a node's contract-kind and blast-radius; `config`/`obs` are no longer mis-bucketed as port-adapters; `authproxy`/`status`/`membackup` read as the source-side services they are.
- Bad: two refinement names to learn; they must be read as side-refinements, never as core-facing directions.
- Neutral: supersedes ADR-0001; the hexagonal axis and the lower-triangular DSM are kept intact.

### Confirmation

Each of the 25 rows in `../README.md` §5 carries exactly one of the six roles; `go-arch-lint check` still shows all edges inward-only (DSM §4 lower-triangular); the six `driven` port-adapters are in bijection with the six ports.
