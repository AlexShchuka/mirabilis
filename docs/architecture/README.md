# mirabilis — Architecture Scheme (target state)

**BLUF.** This is the complete, non-contradictory specification of *what mirabilis is* — every package, at every C4 level — as it should be. It describes the **target state**, not a change list. The repository is brought into conformance with this scheme; the scheme is not a log of fixes.

**Essence (the why — [ADR-0009](adr/0009-system-essence.md)).** mirabilis is a **habitat** for the agent: it builds and tends the dev-container *home* in which Claude Code lives and works. Without an inhabitant the home is nothing; the inhabitant critically depends on the home. The home is nested in a larger environment — the developer host and the open external world — and aims to be the well-built one among the surrounding noise. The system is the **frame** (it shapes what is possible); the agent is the **brain** (it decides). The host side, the in-container side, and the external world are **peers** connected by explicit contracts, not a center over subordinates.

Four strict, machine-checkable artifacts, no ad-hoc notation:

| Artifact | Tool | Role |
|---|---|---|
| `workspace.dsl` | Structurizr DSL (C4) | Context → Container → Component, plus a deployment view for the host/in-container planes |
| `../../.go-arch-lint.yml` | go-arch-lint | Executable **reflexion model** + **fitness function**: every Go package maps to one component; every dependency edge is conformant |
| `adr/*.md` | MADR | Architecture Decision Records — the decisions that *define* the target |
| this file | DSM + traceability matrix | Layer proof + completeness map binding package → role → port → ADR |

The C4 "C1–C4" levels are diagram **zoom levels** (Context → Container → Component → Code, per Brown): a vocabulary for *depth of view*, not a sequence of changes. This document is a description of the target state — the diff against today's tree is derived by comparing the code to this spec, never authored here.

---

## §1 — Method

The spine is correctness from **first principles** — universal laws, mathematics, and proven engineering best practices — then sound code architecture, then maximal information density; topology is not chosen by taste, it falls out of these, and the more statistically-standard a configuration the closer to target (ADR-0009). `general → particular` is realized as the **dependency order**: the foundation (dependency *sinks* — depended upon, depending on little) is described first; the surfaces (the composition root and driving adapters — dependency *sources*) last. This is the same direction as the **hexagonal inward dependency rule** (everything depends inward toward core + ports), with one honest caveat: *driven* adapters are "particular" yet also sit at the dependency foundation (a driven adapter is a sink it implements a port and is imported by use-cases). So the two notions coincide for the core/ports/use-cases/driving spine and diverge only for driven-adapter infra, which is called out explicitly in the layer table (§4).

- **Completeness** is not asserted by the C4 diagrams (C4 deliberately does not require every element be drawn). It is proven by the **reflexion model**: `.go-arch-lint.yml` maps every package to exactly one component; an unmapped package is a literal gap, caught in CI.
- **Non-contradiction** is held by: (1) the package graph is a **DAG** — block-triangular DSM, §4, no cycles, inward-only; (2) conformance rules are **monotone** — they may only get stricter; (3) any decision that revisits an earlier one **supersedes** it via a new ADR.

---

## §2 — C4 levels

- **L1 Context.** Actor: the owner. External systems: Claude Code (the agent that runs in the sandbox), Docker Engine, Anthropic API, Telegram Bot API, LM Studio (host), GitHub.
- **L2 Containers.** `mirabilis` is **one** container — a single multi-mode Go binary. The host vs in-container split is a **deployment** concern, not a container boundary: the same binary runs on the host (control plane: TUI + serve daemon) and inside the dev-container (provision plane: provision/hook/localllm). This is shown in the deployment view, with two deployment nodes instantiating the one container. See [ADR-0002](adr/0002-plane-split.md). Modeling the planes as containers, or the shared code as a "kernel container", is a C4 anti-pattern (a shared library is not a separately runnable container) and is deliberately avoided.
- **L3 Components.** The hexagonal layering (§3), all inside the one container. The component view is a curated C4 view: it elides edges to the common `obs` sink and other shared infrastructure (e.g. `config`) for clarity — the full edge set is in §4 and `.go-arch-lint.yml`. A curated view may omit edges; it must never draw one the model forbids.
- **Code.** The seams are the port interfaces named in the matrix (§5, "Port" column) — the boundaries across which adapters are replaceable (G8).

---

## §3 — Hexagonal classification + node contracts (target)

Every package has exactly one role (ADR-0010). The spine is hexagonal ports & adapters; each of the two sides carries one named refinement, and the inward dependency rule is unchanged — **the core depends on nothing**; everything points inward.

```
  DRIVING SIDE (sources, reached by the root)   CORE         PORTS      DRIVEN SIDE (sinks)
  driving:  tui · tui-leaves · cmd · hooks      pipeline     Runner     driven (implements a port):
  service:  authproxy · status · membackup      bus          Store        exec · secrets · sandbox
  use-case: steps · provision  ───────────────► provModel*   Docker       claudeauth · notify · localllm
                                                inventory*   Notifier   driven (port-less executor):
                                                             Completer    skills · plugins
                                                             TokenSrc   infrastructure (cross-cutting):
                                                                          obs · config · harness · reconcile
```

The six roles (ADR-0010 supersedes the four-role ADR-0001):

- **core** — `pipeline`, `bus`, `provModel*`, `inventory*`: pure domain + value types; depends on nothing.
- **use-case** (application service) — `steps`, `provision`: compose pipelines from core + ports.
- **driving adapter** — `tui`, `tui-leaves`, `cmd`, `hooks`: initiate work on the core.
- **service** — `authproxy`, `status`, `membackup`: a refinement of the **driving** side — long-lived or one-shot *primary actors* started by the composition root (Cockburn); they **consume** a port and implement none. They sit on the source side, not the sink side — they are not driven adapters.
- **driven adapter** — `exec`, `secrets`, `sandbox`, `claudeauth`, `notify`, `localllm` each **implement** exactly one of the six ports (a port↔adapter bijection); `skills` and `plugins` are **port-less** driven adapters — they do external install work through the pipeline, driven by the `provision` use-case.
- **infrastructure** — `obs`, `config`, `harness`, `reconcile`: a refinement of the **driven** side — cross-cutting, injected, depended on by many, behind no port. `config` is the seam through which the owner's desired state enters (it reads/writes `.env` + env); `obs` is the single sink; both are deliberately port-free.

`service` and `infrastructure` are **naming refinements of the two hexagon sides, not new directions the core may depend on** — promoting them to core-facing roles would bend the dependency rule and is the anti-pattern this taxonomy avoids.

**Each node is a black-box contract `{goal, input → output}`** — the port *is* that contract (Parnas information-hiding: hide the decision most likely to change; Meyer design-by-contract: pre/post-conditions; Martin ISP: keep it narrow). A change to one node is provably local **because the contract is its only coupling**, and `go-arch-lint` (the reflexion model / fitness function, §7) fails the build if any import crosses a seam the contract forbids. That is what lets the owner swap a template, tool, or adapter by editing one contract, not the whole graph (G8).

- A **port** is the only legal use-case↔adapter coupling. Ports (6): `Runner`, `Store`, `Docker`, `Notifier`, `Completer`, `TokenSource`. `obs` is injected directly — **not** a port (no `Obs` interface); making it one is a possible future improvement, not this target.
- `*` `provModel` and `inventory` are **target** packages — the contract is specified here, the code is the convergence target (§7).

---

## §4 — Dependency DAG & DSM (layering proof)

Topological layers (Kahn leaf-peeling over `.go-arch-lint.yml` edges). The common component `obs` is reachable by all and is **elided from both the adjacency list and the DSM** to avoid clutter.

```
L0 (foundation)  obs · bus · exec · config · harness · authproxy · reconcile · localllm
L1               secrets · sandbox · pipeline · membackup · tui-leaves
L2               claudeauth · notify · status · prov-plugins · prov-skills
L3               provision · steps
L4               hooks · tui-app
L5 (surface)     cmd
```

`localllm` has no in-project import (arch-lint *permits* `engine-config` but the edge is unused), so it is a true L0 leaf. `bus` and `authproxy` have no non-`obs` in-project import → L0.

**Dependency Structure Matrix (block form).** Rows = source band, columns = dependency band; `▣` = ≥1 edge, `·` = none. Strictly **lower-triangular** ⇒ acyclic, inward-only.

```
   dep→     L0  L1  L2  L3  L4  L5
 src L0      ·   ·   ·   ·   ·   ·
 src L1      ▣   ·   ·   ·   ·   ·
 src L2      ▣   ▣   ·   ·   ·   ·
 src L3      ▣   ▣   ▣   ·   ·   ·
 src L4      ▣   ▣   ▣   ▣   ·   ·
 src L5      ▣   ▣   ▣   ▣   ▣   ·
```

**Adjacency (exact target edges; common `obs` edge and vendor elided):**

```
secrets     → exec
sandbox     → exec, config
pipeline    → exec
membackup   → exec
tui-leaves  → bus
claudeauth  → secrets
notify      → config, secrets
status      → sandbox
prov-plugins→ pipeline, reconcile
prov-skills → config, pipeline, reconcile
provision   → exec, config, harness, pipeline, prov-skills, prov-plugins
steps       → exec, secrets, config, harness, sandbox, claudeauth, authproxy, pipeline, notify
hooks       → exec, config, notify, provision
tui-app     → bus, exec, pipeline, steps, tui-leaves
cmd         → (composition root: all of the above)
localllm    → (none in-project)
```

This is exactly the edge set of `.go-arch-lint.yml` (minus the unused `localllm→config` permission). The two `*` target packages add: `inventory → (none)` and `provModel → (none)`; consumers `cmd`, `steps`, `provision` gain an edge to them.

---

## §5 — Traceability matrix (completeness)

23 current packages (= the 23 `.go-arch-lint.yml` components) **+ 2 target packages** = **25 component rows**, each carrying exactly one of the six roles (ADR-0010). The 6 ports are interface types living inside existing packages (the "Port" column), rendered as separate boxes in the DSL component view but not counted as packages. Every row has a role + verdict cell — that is the completeness guarantee.

| Component | Role | Runs in | Layer | Go package | Port | ADR |
|---|---|---|---|---|---|---|
| pipeline | core | both | L1 | `internal/engine/pipeline` | — | 0010,0005 |
| bus | core | control | L0 | `internal/bus` | — | 0010 |
| provModel \* | core (target) | both | — | `internal/engine/provision/model` | — | 0010,0005 |
| inventory \* | core (target) | both | — | `internal/engine/inventory` | — | 0006,0010 |
| steps | use-case | control | L3 | `internal/engine/steps` | uses Runner/Docker/Store/TokenSource/Notifier | 0010,0005 |
| provision | use-case | provision | L3 | `internal/engine/provision` | uses Runner | 0010,0005,0006 |
| tui-app | driving | control | L4 | `internal/tui/app` | — | 0010 |
| tui-leaves | driving | control | L1 | `internal/tui/{screens,components,frame,router,strings,styles,a11y}` | — | 0010 |
| cmd | driving | both | L5 | `cmd/mirabilis` | composition root | 0003,0010 |
| hooks | driving | provision | L4 | `internal/hooks` | — | 0008,0010 |
| authproxy | service | control | L0 | `internal/engine/authproxy` | uses TokenSource | 0007,0010 |
| status | service | control | L2 | `internal/engine/status` | uses Docker (via sandbox) | 0004,0010 |
| membackup | service | control | L1 | `internal/engine/membackup` | uses Runner (Save fn, no port) | 0003,0010 |
| exec | driven | both | L0 | `internal/engine/exec` | implements Runner | 0003,0010 |
| secrets | driven | control | L1 | `internal/engine/secrets` | implements Store | 0003,0007 |
| claudeauth | driven | control | L2 | `internal/engine/claudeauth` | implements TokenSource | 0007 |
| sandbox | driven | control | L1 | `internal/engine/sandbox` | implements Docker | 0003 |
| notify | driven | control | L2 | `internal/engine/notify` | implements Notifier | 0003 |
| localllm | driven | provision | L0 | `internal/engine/localllm` | implements Completer | 0007 |
| prov-skills | driven (port-less) | provision | L2 | `internal/engine/provision/skills` | — | 0007,0010 |
| prov-plugins | driven (port-less) | provision | L2 | `internal/engine/provision/plugins` | — | 0006,0007,0010 |
| obs | infrastructure | both | L0 | `internal/obs` | — (concrete sink) | 0004,0010 |
| config | infrastructure | both | L0 | `internal/engine/config` | — (desired-state seam) | 0010 |
| harness | infrastructure | both | L0 | `internal/engine/harness` | — | 0007,0010 |
| reconcile | infrastructure | provision | L0 | `internal/engine/provision/reconcile` | — | 0010 |

\* target package: the contract is specified here, the code is the convergence target. `go-arch-lint` maps the **23 realized** packages today; this matrix describes the **25-component** target, and the two target rows gain their `.go-arch-lint.yml` component when their code lands (§7). The DSL renders **31 boxes** = these 25 package-components + the 6 port interface elements.

---

## §6 — Invariants (target; each has a mechanical home)

Meta-goals **G0–G8** and invariants **I1–I13** are defined in `../../CLAUDE.md` and remain the contract. Architecture-load-bearing mapping:

| Invariant | Architectural statement | Mechanical home |
|---|---|---|
| G2 | engine never imports tui/bus/bubbletea; behaviour never leaks into sandbox nodes | depguard `engine-no-tui` |
| G3 | one responsibility per component; a bug localizes to one package | go-arch-lint boundaries + review |
| G4 | all tunables in `config`; code only reads them | only `config` writes `.env` |
| G5 | one observability sink: everything logs via `obs` | forbidigo `^os\.Stderr$` (excl. obs, cmd, hooks, tests) |
| G8 | adapter swap = new file + one registration | port interfaces (§5) + demo-adapter test |
| I1 | real Anthropic token never in the container | e2e token-grep; proxy holds token, container gets per-session key |
| I2 | UI thread does no I/O | forbidigo in `tui/**` except `tui/app` Cmd ctors |
| I3 | every non-terminal step idempotent: Run ⇒ Check=true | `pipeline.Contract` test |
| I4 | every command spawns only via `Runner` and emits `started{Argv}` | forbidigo (raw `os/exec` banned) |
| I7 | a secret lives in one backend per platform; old migrated then deleted | secrets migration unit |
| I8 | all UI strings in `tui/strings`, English | grep test |
| I12 | a single node's failure never blocks the menu; shows `degraded` | fault-injected adapter unit |
| I13 | one sink; `obs`, `cmd`, `hooks` (+ `_test.go`) are stderr-exempt | forbidigo exclusions `.golangci.yml` |
| §4.2 | `tui/{screens,components,frame,router}` never import `internal/engine` | depguard `tui-leaves-no-engine` |
| D10 | no comments in code/config; prose only in `.md` | `no-config-comments` CI + review |

---

## §7 — Conformance (completeness + non-contradiction)

- **Reflexion / completeness.** `.go-arch-lint.yml` is the high-level model. `go-arch-lint check` proves: every package maps to a component (no *absence*); every actual import is an allowed edge (no *divergence*). A new package with no component is a build failure.
- **Fitness functions.** `go-arch-lint check` + `golangci-lint run` (depguard, forbidigo, errcheck, unused) + `go test -race` + `bats` — executable, continual. Not-green = not-conformant.
- **Monotonicity.** Conformance rules only tighten; removing an allowed exception is permitted, adding a violation is not; each tightening is an ADR.
- **ADR supersession.** A decision is never edited in place after acceptance; a reversal is a new ADR marked `superseded` referencing the old number.

Target packages (`inventory`, `provModel`) are not in `.go-arch-lint.yml` until their code exists; each gains its component when its code lands, at which point the reflexion model maps 25/25. The live arch-lint config is never loosened ahead of the code, so the doc may describe more components than are executable today — that gap is precisely the convergence vector, not a divergence.

---

## §8 — ADR index

| # | Decision |
|---|---|
| [0001](adr/0001-hexagonal-decomposition.md) | Hexagonal (ports & adapters) is the classification axis; every package has exactly one role — **superseded by [0010](adr/0010-role-taxonomy.md)** on the role count |
| [0002](adr/0002-plane-split.md) | One container; host/in-container planes are deployment nodes (not containers, not a kernel library-container) |
| [0003](adr/0003-ports-and-adapters.md) | Ports are the only use-case↔adapter coupling; adapters are replaceable (G8) |
| [0004](adr/0004-single-obs-sink.md) | One observability sink; status + logs converge on `obs` (G5) |
| [0005](adr/0005-pipeline-core.md) | The pipeline FSM + idempotent Check→Run→Check is the domain core (I3, G7) |
| [0006](adr/0006-inventory-contract.md) | Inventory configured-sentinel is the host→container provisioning contract (D2/D7) |
| [0007](adr/0007-locked-boundaries.md) | Locked boundaries: gh-skill install (D1), token chain (I1), egress edges (§5) |
| [0008](adr/0008-conformance-as-code.md) | go-arch-lint = reflexion model + fitness function; no comments in code (D10) |
| [0009](adr/0009-system-essence.md) | System essence: a habitat for the agent; frame-not-brain; first-principles spine; peers across the wall |
| [0010](adr/0010-role-taxonomy.md) | Six-role taxonomy (core, use-case, driving, service, driven, infrastructure); service/infrastructure are named refinements of the two hexagon sides, not core-facing directions; each node is a black-box contract enforced as a port + go-arch-lint (supersedes 0001) |
