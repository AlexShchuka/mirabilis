---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# System essence: mirabilis is a habitat for the agent

## Context and Problem Statement

ADRs 0001–0008 decide *structure*. They float free of intent unless one decision records *what the system is for* and the philosophy from which the rest derive (G0: architecture soundness is judged by what it does to the whole graph first). This ADR is that root; the others serve it.

## Decision Drivers

- A single coherent intent must anchor every structural choice (G0, G1).
- The owner reasons about the system as a whole, not as a pile of parts; the scheme must read purpose-first.

## Decision Outcome

The essence, four facets:

1. **Habitat, not tool.** mirabilis is the *environment* — the *home* — in which the agent (Claude Code) lives and works. Without an inhabitant the home is nothing; the inhabitant critically depends on the home. The home does not stand in a desert: it is nested in a larger environment — the developer host and the open external world (the internet) — and aims to be the well-built one among the surrounding noise.

2. **Frame, not brain.** The system is the *frame*: it shapes what is possible (mechanism, constraints, affordances) and influences the agent by bounding the space, never by deciding within it. The agent is the brain. The system carries no goals of its own (G2: the sandbox provisions, the harness behaves; behaviour never leaks into the frame).

3. **Spine = first principles.** The backbone is correctness derived from universal laws, mathematics, and proven engineering best practices; below that, sound code architecture; below that, maximal information density. Topology is not chosen by taste — it falls out of these. The more statistically-standard (proven) a configuration, the closer to target.

4. **Peers across the wall.** The host-side controller, the in-container side, and the external world are *peer* entities connected by explicit contracts — not a privileged center over subordinates. Per facet 3 the proven single-artifact form is kept (one binary, two deployment locations, a clear contract across the host↔container wall); a hard split into two binaries remains open if it becomes statistically warranted.

### Consequences

- Good: every structural ADR traces to one intent; the scheme reads purpose-first; "kept unchanged" is legible as serving the essence, not as oversight.
- Bad: the essence is partly philosophical — it constrains and explains (the *why*) but is not itself machine-checkable (the *what* is, in 0001–0008).
- Neutral: supersedes nothing; it is the root the other ADRs serve.

### Confirmation

ADR-0002 + 0007 realize facets 2 and 4 (frame, peers, contracts); ADR-0005 realizes facet 3 at the core; ADR-0003 + 0004 keep the frame replaceable; `README.md` §1–§2 lead with the habitat framing.
