---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# Hexagonal (ports & adapters) is the component classification axis

## Context and Problem Statement

The architecture scheme must classify *every* package — a complete partition with no node left implicit. The C4 model gives leveled decomposition but deliberately does not require completeness (you draw only the levels that "add value"). A separate, total classification axis is needed, and it must coincide with the `general → particular` (dependency-sink → dependency-source) ordering so completeness and sequencing share one axis.

## Decision Drivers

- Completeness: every component gets exactly one role (G0, G1).
- One responsibility per node, bug localizes to one package (G3).
- Replaceable nodes (G8) require an explicit boundary between logic and its outside.
- The classification must align with the dependency order so the DAG is the proof.

## Considered Options

- Hexagonal ports & adapters (Cockburn): domain-core / port / driven-adapter / driving-adapter, plus an application-service layer for the use-cases.
- Classic n-tier layering (presentation/business/data).
- C4-only, no role partition.
- Ad-hoc per-package labels.

## Decision Outcome

Chosen: **hexagonal ports & adapters**, extended with an explicit **application-service** layer for the use-cases (`steps`, `provision`). It is a total partition; its inward dependency rule aligns with the topological order of the package DAG.

### Consequences

- Good: every package in the traceability matrix has exactly one role; the dependency DSM is lower-triangular; G8 replaceability falls out of the port boundary.
- Bad: ports are interface types, not packages, so the matrix lists them in a "Port" column rather than as rows — they appear as boxes in the DSL component view but are not counted among the 25 package-components.
- Neutral: target grouping may differ from the current package layout; the rewrite re-groups toward roles.

### Confirmation

Each of the 25 component rows in `../README.md` §5 carries exactly one role; `go-arch-lint check` shows all edges inward-only (DSM §4 lower-triangular).
