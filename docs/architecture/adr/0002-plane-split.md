---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# One container; host/in-container planes are deployment nodes

## Context and Problem Statement

`mirabilis` is a single multi-mode Go binary: it runs host-side (TUI, serve daemon) and inside the dev-container (provision, hook, localllm), and it provisions a third thing — the dev-container itself. The architecture must make the host↔container boundary explicit (meta-goal G2: the sandbox provisions, the harness behaves; behaviour must not leak into sandbox nodes) **without misusing C4 notation**.

## Decision Drivers

- G2: the host↔container line is load-bearing.
- I1: the real token lives host-side; only a per-session key crosses into the container.
- C4 correctness: a strict, industry-standard model with no anti-patterns.

## Considered Options

- **A. Planes as C4 containers + a shared-kernel container** the plane binaries embed.
- **B. One C4 container (the binary); the planes are deployment nodes.**
- **C. Split into two separate binaries (host-agent and in-container-agent).**

## Decision Outcome

Chosen: **B**. There is one C4 container — the `mirabilis` binary, with all components grouped by hexagonal role. The host↔container split is a **deployment** concern: the deployment view has a *control plane* node (host) and a *provision plane* node (inside the dev-container), each instantiating the one container. The dev-container is the deployment node that is the security boundary and that also hosts the Claude Code instance.

This **supersedes** the initial draft (option A). An adversarial review flagged option A as the C4 "shared-library-as-container" anti-pattern (Simon Brown: shared libraries are reusable code, not separately deployable containers; containers communicate via inter-process mechanisms, which the single embedded binary does not). The host/in-container distinction is, by C4 definition, a deployment-node distinction — the same artifact run in two locations.

### Consequences

- Good: strict C4; no false in-process cross-plane relationships; the host↔container boundary is explicit and correctly located in the deployment view.
- Bad: the L2 container view is a single box — acceptable for a monolithic CLI; the architectural richness lives at L3 (components) and in the deployment view.
- Neutral: option C (a hard two-binary split) remains open and would promote the two deployment nodes into two real containers without changing the component model.

### Confirmation

`workspace.dsl` has exactly one `container`; the planes appear only as `deploymentNode`s each with a `containerInstance` of it; no relationship couples a control-only component to a provision-only component in-process.
