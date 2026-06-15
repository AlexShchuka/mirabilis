# Provision → Inventory-Driven Sandbox: Refactor Plan (C1–C4)

Status: **design doc, owner-review — iteration 4**. Encoding: English + first-order logic where a
real formal object exists; per stage an ordered, machine-executable step list. Per-claim tag
`[FACT file:line]` (verified against the working tree at `#145`) · `[ASSOC]` (re-verify at impl) ·
`[HYPO]` · `[Q]`. Anti-sycophancy: a claim carries adjacent evidence or a tag; no evaluation without an
anchor; a judgment is tagged `[eval]`, not stated as fact.

Path convention: `skills.go` / `plugins.go` below mean the **installer subpackages**
`internal/engine/provision/skills/skills.go` and `internal/engine/provision/plugins/plugins.go`
(the top-level `provision/skills.go` is the 32-line step wrapper). All other files are repo-root-relative; `Dockerfile` = `.devcontainer/Dockerfile`.

Scope law (G2): `M` (sandbox) provides a *mechanism*; *when/whether* to use it is `N` (harness). A Δ that
decides behaviour is mis-placed here. Mission (INV-MISSION): every Δ justified by
⟨truth, capability, self-sufficiency, evolvability⟩ — token/LOC economy is a consequence, never a rationale.

## §-1 — Locked decisions (owner sign-off)

```
D1  install_s canon = gh skill install   (KEEP #145).  git-clone path REJECTED.        [FACT skills/skills.go:54-62]
D2  inventory encoding = configured-sentinel  {version:int, configured:bool, skills:[], plugins:[]}
      rationale: make the third state EXPLICIT, not an implicit nil-vs-empty pointer trick.
      in-repo precedent: plugins.Plan.Configured bool  [FACT plugins/plugins.go:21,44].
      ([Q] k8s api-conventions use a nilable *[]T for unset≠empty; we choose an explicit sentinel
       field for Go nil-safety — divergence is deliberate, owner-signed.)
D3  seam name = Inventory / inventory.json   (Sandbox.Desired is taken: git short-HEAD fingerprint)  [FACT sandbox.go:67]
D4  migration = one-shot cutover ; NO coexistence/dual-read window  (single-owner, non-prod repo; rollback = git revert)
D5  validation = hand-rolled Validate() against the catalog (single source) ; NO JSON Schema
D6  darwin/platform robustness ∈ C2  (the .mirabilis host↔container file contract)
D7  Inventory type lives in a dedicated leaf package, not engine/config        [k8s api-types pattern]
D8  C4 interfaces = consumer-defined minimal (each step its own) ; shared only for tiny universal roles (Logger)  [Go CodeReviewComments]
```

Inventory wire contract (D2, D7) — package `internal/engine/inventory`, the merge-ready type (comment-free per D10; field semantics in prose):
```go
package inventory

type Inventory struct {
	Version    int      `json:"version"`
	Configured bool     `json:"configured"`
	Skills     []string `json:"skills"`
	Plugins    []string `json:"plugins"`
}
```
- `Version` — fail-fast GUARD, not a migration mechanism: a reader on `Version > known` fails loud
  (engineering-bar). D4 forbids a dual-read window ⇒ no fallback format exists; the field guards, it does not migrate.
- `Configured` — `false ∨ absent ⇒ NotConfigured` (INV-EMPTY≠UNSET). In-repo precedent: `plugins.Plan.Configured`  [FACT plugins/plugins.go:21,44].
- `Skills` — selected group-names ∈ catalog (opt-in, mirrors `.env SKILLS`) ; `[] = explicit none`.
- `Plugins` — **enabled** plugin-names ∈ catalog (opt-in). The legacy source is opt-OUT (`.env PLUGINS_DISABLED`),
  so the host seeds `Plugins = pluginCatalog − PLUGINS_DISABLED`  [FACT plugins/plugins.go:31-44]. The implicit
  `neuro-matrix@neuro-matrix` (BuildPlan adds it unless harnessSkip)  [FACT plugins/plugins.go:15,33-35]  stays
  harness-governed via `harnessChoice` (kept global) — NOT stored in inventory.
  ([Q] alt rejected: store `PluginsDisabled` to mirror legacy 1:1 — lossless seed but carries the opt-in/opt-out
   asymmetry into the new contract; rejected for G1 coherence. Owner may veto.)

---

## §0 — Ontology

```
Stages      Σ = {C1, C2, C3, C4}                       ordered, bottom-up
Templates   T = {code, …}                              |T| open; today |T_impl| = 1 (implicit "code")
Catalog     skills.txt (grouped: "name repo skill…"), plugins.txt, marketplaces.txt   [FACT config.go:102-145]
Selection   host .env : SKILLS (csv group-names), PLUGINS_DISABLED (csv)               [FACT config.go:76,80]
Legacy bus  D = container ~/.claude dotfiles written via host-driven docker-exec:
              {.mirabilis-skills, .mirabilis-plugins-disabled, .mirabilis-harness}     [FACT carry.go:16,17,19; written apply.go:77-86]
            (.mirabilis-theme @carry.go:18 interleaves the consts but is NOT in the seam)
Inventory   inv = ⟨version, configured, Skills:[…], Plugins:[…]⟩   resolved, opt-in   [D2]

resolve   : T → inv                          [LOCATION host]                           [FACT facade.go:199-212 resolveRepo + Dockerfile:138]
validate  : inv → inv | err                  Validate() vs catalog [D5]
install_s : Skill  → gh skill install <repo> <skill> --agent claude-code --scope user --force  [FACT skills/skills.go:54-62]
install_p : Plugin → claude plugin install <name> --scope user                         [FACT plugins/plugins.go:99]
              prereq marketplace add                                                    [FACT plugins/plugins.go:89]
state_s   : gh skill list --agent claude-code --scope user --json skillName,sourceURL  [FACT skills/skills.go:66]
Bootable(x), Green(x) : repo state x launches ; go test -race ∧ golangci-lint ∧ go-arch-lint ∧ bats pass
Reads(step, src), Writes(actor, file)
```

`[FACT docker-compose.yml:41]` bind-mount `./.mirabilis ↔ /workspace/.mirabilis` exists.
`[FACT Dockerfile:133,138]` `MIRABILIS_REPO=/opt/mirabilis`; container config root `= /opt/mirabilis/config`
(image-baked, ≠ bind-mount) ⇒ `resolve` MUST run host-side.
`[FACT facade.go:199-212]` host `resolveRepo()` falls through to `os.Executable()` (container-only
`MIRABILIS_REPO` unset on host) ⇒ host reads its live working tree; config is read via `config.New(repo+"/config")` (`facade.go:218`).
`[FACT provision.go:28-37]` `Deps` = 8 fields; "god-object" is `[eval]`, not a mechanical fact.

---

## §1 — Invariants (preserved by every stage; each maps to a repo invariant home)

```
INV-SAFE            ∀ pr : Bootable(after(pr)) ∧ Green(after(pr))                         (Engineering bar: not-green=not-done)
INV-EXPAND-CUTOVER  C1,C3,C4: expand→migrate→contract (ParallelChange) ; C2: expand→CUTOVER (D4, no window)
INV-RECONCILE       Check recomputes inv-vs-actual each launch ; level-triggered ; idempotent   (G7, I3)
INV-EMPTY≠UNSET     configured=⊥ ∨ absent ⇒ NotConfigured ⇒ explicit-log ; ¬(empty ⇒ silent-healthy)   (explicit-sentinel, D2)
INV-DISJOINT        ∀ ci≠cj ∈ ECON : channel(ci) ∩ channel(cj) = ∅                        (PRESERVE)
INV-I1 (sec)        token ∉ content(inventory.json) ; content(inventory.json) ⊆ {skill,plugin names}    (I1)  [FACT SECURITY.md:70]
INV-DARWIN          ∀ f ∈ .mirabilis : Writes(host,f) ∧ Reads(container,f) ⇒ Readable(container,f)    [0644 ∨ uid(container)=uid(host)]
INV-G2              resolve ∈ mechanism(M) ; "which template / when" ∈ N                  (G2)
```

`[FACT SECURITY.md:70]` `.mirabilis/` already carries non-secret host↔container facts (chat-id);
`inventory.json` (names only, 0644) is the same class ⇒ INV-I1 holds. `session-key` (0600) is a **separate**
secret artifact governed by INV-DARWIN, not by the inventory seam.

Note (D4 vs ParallelChange): the fallback/compatibility window is what makes ParallelChange safe. C2 deletes
it deliberately — C2 is therefore an honest **hard cutover** (bootable via PR-atomicity + green gates +
`git revert`), NOT parallel-change. Only C1/C3/C4 keep the full expand→migrate→contract triad.

---

## §2 — Target architecture

```
T --resolve(host)--> inv --validate(host)--> --fsio.WriteJSON(0644)--> ./.mirabilis/inventory.json
                                                          | (bind-mount ; mkdir .mirabilis host-first)
                                                          v
                              container reads /workspace/.mirabilis/inventory.json
                                                          | reconcile  (level-triggered ; INV-EMPTY≠UNSET)
                                                          v
              ∀ s∈inv.Skills install_s(s) ; ∀ p∈inv.Plugins install_p(p)   (idempotent ; gh-skill canon D1)
```

Critic-fix (load-bearing) `[FACT Dockerfile:138, facade.go:199-212]`:
```
config_root(container) = /opt/mirabilis/config  (image)  ≠  bind-mount
  ⇒ resolve runs host-side (host reads config/templates/* live from the working tree)
  ⇒ container reads ONLY inventory.json ; ¬Reads(container, catalog)
  ⇒ "image-baked config" ceases to constrain template editing
```

Seam target after C2:
```
|{ f ∈ D : Reads(provision, f) }| = 0          (skills, plugins-disabled migrated to inventory.json)
skills/plugins steps wired exactly once per launch (carryStart redundant wiring removed — C2-step5)
harness path unchanged: harnessChoice() KEEP (harness global)   [FACT carry.go:97-103]
```

---

## §3 — Stage transitions (ordered machine-executable steps + touch-matrix)

Cell = system Δ; ∅ = untouched. Each numbered step is one reviewable commit (diff minimal).

### C1 — `engine/fsio` leaf + DRY  (install mechanism UNCHANGED — D1)
```
1  + internal/engine/fsio {ReadJSON, WriteJSON(atomic, 0644), CopyFile, PathExists, ReadLines, ReadLineSet}
     bodies moved from jsonio.go:9-57 + carry.go:115-146 ; old = thin wrappers        [FACT]
2  .go-arch-lint.yml: + component `engine-fsio` (leaf) BEFORE caller switch            (else depguard violation)
3  switch callers → fsio: engine-provision (jsonio + carry helpers, incl. rtk.go/cavshrink.go which already use readJSON/writeJSON) ; engine-notify (queue.go:141 writeAtomic) ; cmd (flock, sessionkey)
4  delete wrapper dups
   engine-localllm ∅  (os.Executable only ; ¬json/file-content IO)
```
| system | Δ | tag |
|---|---|---|
| engine-provision | jsonio.go {readJSON,writeJSON,copyFile} + carry.go {readSet,readLines,exists} → engine-fsio (exported leaf) ; rtk.go/cavshrink.go callers switch | `[FACT jsonio.go:9-57, carry.go:115-146]` |
| engine-notify | queue.go writeAtomic → fsio | `[FACT queue.go:141]` |
| cmd | flock / sessionkey file-prims → fsio | `[FACT flock_unix.go, sessionkey.go:21]` |
| engine-localllm · tui · container · security | ∅ | |
```
ParallelChange: expand(1-2 ; old=wrappers) → migrate(3) → contract(4).
Bootable ⊤ (pure-fn move, behaviour ≡).  LOC: deletes dup helper bodies; expect non-positive net [verify at impl].
NOTE: iteration-1 C1-Δ4 "golang→plugin / drop gh-skill / git-clone" is DELETED — contradicts merged #145 (D1).
```

### C2 — Inventory seam + redundant-wiring removal + `.mirabilis` contract (incl. darwin — D6)
```
1  + package internal/engine/inventory : type Inventory (§-1) + Validate() vs catalog (D5)   [D7]
2  host LaunchSteps (facade.go:123-125): mkdir .mirabilis (host-first) ; SEED inv — Skills←.env SKILLS (opt-in direct) , Plugins←pluginCatalog−.env PLUGINS_DISABLED (opt-out→opt-in normalize) , neuro-matrix implicit stays harness-governed (¬inventory) ; validate ; fsio.WriteJSON(0644) inventory.json   (G7: no selection loss)
3  container reconcile reads inventory.json ; INV-EMPTY≠UNSET — repairs empty=healthy mask   [FACT skills/skills.go:19-22, 41-44]
     absent inventory.json ⇒ NotConfigured ⇒ explicit log (default code, G6) ; malformed ∨ Version>known ⇒ fail loud (engineering bar)
4  apply.go (steps) reads inventory.json, NOT dotfiles+env(MSKILLS/MDIS) ; write-path ∧ Check migrate together   [FACT apply.go:30-86]
5  remove pluginsStep(carry.go:44), skillsStep(carry.go:45) from carryStart ; KEEP harnessStep(carry.go:43)   [FACT carry.go:43-45]
6  cutover (D4): delete 3 seam dotfiles + env-write-path , SAME PR , no fallback read
7  darwin (D6): session-key readable by container (sessionkey.go:21 ; 0600→INV-DARWIN)
```
| system | Δ | tag |
|---|---|---|
| inventory (new leaf) | type + Validate() | `[D7]` |
| cmd (host) | LaunchSteps writes inventory.json ; mkdir .mirabilis ; seed from .env | `[FACT facade.go:123-125, config.go:76,80]` |
| engine-steps | apply.go reads inventory.json ; Check migrates with write-path | `[FACT apply.go:30-86]` |
| engine-provision | carryStart trimmed (drop plugins@44 / skills@45) ; harnessChoice kept | `[FACT carry.go:43-45, 97-103]` |
| container/security | bind-mount write ; mkdir host-first ; INV-I1 ∧ INV-DARWIN | `[FACT compose:41, SECURITY.md:70, sessionkey.go:21]` |
| economy · notify · tui | ∅ | |
```
expand→CUTOVER (D4) ; Bootable ⊤ (PR-atomic) ; rollback = git revert.
REDUNDANT WIRING (corrected from "2× install"): Launch() runs provision-start (→carryStart→plugins/skills steps)
AND newPluginsApply/newSkillsApply (→ provision --phase plugins/skills → same steps).  [FACT steps.go:48-62, apply.go:84]
The installers are idempotent (reconcile vs `gh skill list` / `claude plugin list`), so install_s/install_p run ONCE;
the cost of the double wiring is (a) a redundant docker-exec + Check round-trip, and (b) an ordering hazard —
carryStart fires before the apply step provides the selection, so the first pass runs on stale/empty input.
Removing it is justified by clarity (G1) + the ordering fix, NOT by a double-install (that claim was overstated).
```

### C3 — template layer (host-resolve + validate) — user-visible
```
1  + config/templates/<name>/{skills.txt, plugins.txt}        (greenfield-additive; nothing reads it yet)
2  + config.ResolveTemplate(name) → inv          [LOCATION host]   [FACT facade.go:199-212 host has live working tree]
3  validate resolved inv vs catalog (D5) BEFORE write           [helm template + kubeconform analogue]
4  engine/steps/forms.go (host wizard config, NOT tui): {plugins,skills} MultiSelect groups → single-select template group + KEEP stacks ; default=code   [FACT steps/forms.go:92-159]   [devcontainer Templates(single) vs Features(multi)]
5  host saveInventory resolves chosen templateID (not raw SKILLS/PLUGINS)
6  redirect catalog reads, THEN move flat skills.txt/plugins.txt → templates/code/. Call sites to redirect: config.SkillsTxt()/PluginsTxt() (config.go:35-36), ReadSkillGroups (config.go:131), ReadPluginCatalog (config.go:102)
```
| system | Δ | tag |
|---|---|---|
| config | + templates/ dir + ResolveTemplate (new) | `[ASSOC insertion region config.go:26-36]` |
| engine-steps | forms.go single-select template + keep stacks ; drop 2 multi-groups | `[FACT steps/forms.go:92-159]` |
| cmd | resolve templateID host-side | `[FACT facade.go:199-212]` |
| container | ∅ (resolve host-side ; image-baked config irrelevant) | `[FACT Dockerfile:138]` |
| tui · economy · notify · provision | ∅ | |
```
ParallelChange: expand(templates/code ≡ flat ; default code) → migrate(menu→template→host-resolve) → contract(rm flat + collapse multi-wizard).
Bootable ⊤ (default code ≡ today).
```

### C4 — god-object teardown + dispatch  (each step ties to a named god-object defect; G1/G3)
```
1  Deps (8 fields, provision.go:28-37) → consumer-defined minimal interfaces per step ; shared only Logger   [D8; G8]
     pitfall: do NOT replace god-struct with god-interface, and do NOT pre-define interfaces before a call site uses them
2  dissolve carry/provision builders ; dedup headroom.scriptOK vs carry.scriptOK   [FACT headroom.go:132, carry.go:79]
3  cmd god-object: main.run → dispatch-table ; single newDeps factory (the inline f.deps literal is duplicated logic)   [FACT facade.go:89-99]
4  notify queue.go split {types|io|gc} ; tunables → config (G4).  funcs = WriteJob(38), ReadJob(52), writeAtomic(141) ; watcher.go = 2 constants   [FACT queue.go, watcher.go:18-19]
5  tui/app god-object (20 fields) : waiting/menuAction/busy → substructs (internal, behaviour ≡)   [FACT app.go:49-70]
6  tighten .go-arch-lint.yml (provision-skills/plugins/reconcile already components post-#145) ; drop plumbing tests
```
```
DROPPED from iteration-1: "hook stderr-writes → obs sink (I13)".  Refuted: forbidigo's os.Stderr rule
DELIBERATELY excludes internal/hooks (.golangci.yml:30-31,86-89; AGENTS.md I13), because hooks run as
separate short-lived `mirabilis hook <name>` processes in-container with no reachable host obs sink. Routing
them to obs would reverse a recorded decision (I13) without new evidence — out of scope.

ParallelChange: internal refactor ; external behaviour ≡ ; Green per commit.
INV-DISJOINT preserved by construction: each step depends only on the narrow interface it uses ⇒ no cross-step import.
```

---

## §4 — Safety theorems

```
T-BOOT   ⊢ ∀ Cᵢ : Bootable(after(Cᵢ))   by PR-atomicity (C2 cutover) ∧ expand-window (C1,C3,C4) ∧ behaviour-≡(C1,C4) ∧ default=code(C3)
T-SEC    ⊢ INV-I1 : content(inventory.json) ⊆ names ; ⊉ secret ; 0644 non-secret ; session-key separate (INV-DARWIN)
T-NOMASK ⊢ ¬(empty ⇒ silent-healthy)   by INV-EMPTY≠UNSET (configured-sentinel D2)   — repairs skills/skills.go:19-22,41-44
T-1WIRE  ⊢ skills/plugins wired once per launch   by C2-step5 (carryStart trim)   — removes redundant invocation (steps.go:48-62 + apply.go:84) ; install was already once (idempotent)
T-ECON   ⊢ INV-DISJOINT after C1..C4   C1=fsio-only ; C4=consumer-narrow-interfaces ⇒ no cross-step import
```

---

## §5 — Out of scope (recorded decisions; reopen needs new evidence, not association)

```
authproxy upstream = api.anthropic.com (hardcoded)        SECURITY.md / AGENTS.md — intentional token chain
localllm edge = host.docker.internal:1234                 SECURITY.md / AGENTS.md — intentional egress edge
hooks → os.Stderr                                          I13 deliberate exemption (.golangci.yml:30-31,86-89) — NOT rerouted
framework G0–G8 / I1–I13 / engine-tui split / M-N split   healthy; not refactored
headroom-upstream + create/start markers                  different mechanism; NOT the seam
memory + MemoryCategories                                 global, NOT template-scoped
```

---

## §6 — Verification (each new invariant gets a mechanical home — repo rule)

```
∀ Cᵢ : Green ⟺ go test -race ./... ∧ golangci-lint run ./... ∧ go-arch-lint check ∧ bats test/
C0 (prereq): DONE — origin/main = #145 (gh-skill canon, grouped skills.txt, subpackages). D1 reconciled.

mechanical homes:
  INV-EMPTY≠UNSET / T-NOMASK : unit on inventory reconcile — configured=⊥ ⇒ NotConfigured (not healthy) ; configured=true,[] ⇒ install 0
  Inventory.Validate (D5)    : unit — name ∉ catalog ⇒ err ; round-trip JSON (nil vs [] preserved via Configured)
  T-1WIRE                    : bats e2e with a spy/fake Runner counting `gh skill install` invocations = 1 per launch
  INV-DARWIN                 : e2e on macOS runner — host writes .mirabilis/* ; container reads succeed
  absent/malformed inventory : unit — absent ⇒ default code + log ; Version>known ⇒ fail loud
  C1 engine-fsio leaf        : go-arch-lint component rule (no outgoing deps)
  C4 narrow interfaces       : go-arch-lint deny on cross-step import ; I13 unchanged (hooks stderr still exempt)
  full e2e (I9/G7)           : launch ⇒ template ⇒ skills∪plugins installed once ⇒ relaunch zero-change
```

---

## §7 — Ledger

```
FACT (verified vs tree @#145):
  compose:41 ; Dockerfile:133/138 ; provision.go:28-37 ; carry.go:16-19/43-45/79/97-103/107-113/115-146 ;
  skills/skills.go:19-22/41-44/54-62/66 ; plugins/plugins.go:22/44/89/99 ; config.go:35-36/76/80/102/106-145/131 ;
  facade.go:89-99/123-125/199-212/218 ; sandbox.go:67 ; jsonio.go:9-57 (WriteJSON 0644 @:44) ;
  steps.go:48-62 ; apply.go:30-86/77-86 ; steps/forms.go:92-159 ; SECURITY.md:70 ; sessionkey.go:21 ;
  headroom.go:132 ; queue.go:38/52/141 ; watcher.go:18-19 ; app.go:49-70 ; .golangci.yml:30-31/86-89 (I13 hooks exempt).

CORR (iteration 3 → 4, adversarial pass 2):
  carry.go trim refs 44/45/46 → 43/44/45 (harness@43 keep, plugins@44 / skills@45 remove) ;
  .env→inventory seeding corrected — plugins opt-OUT in source, normalized to opt-in enabled (pluginCatalog−PLUGINS_DISABLED) ; neuro-matrix implicit stays harness-governed ;
  Inventory Go type stripped of comments (D10), semantics in prose ; Version reframed = fail-fast guard (not migration) ;
  Dockerfile path = .devcontainer/Dockerfile ; D3 gloss = git short-HEAD (not "image") ; .golangci.yml ref @86-89.

CORR (iteration 2 → 3, from adversarial refutation):
  install_s anchor = skills/skills.go subpackage (not provision/skills.go) ; gh skill list @:66 ; plugins :89=marketplace add /:99=install ;
  "install 2×" → REDUNDANT WIRING (idempotent-skipped 2nd pass + ordering hazard), not double install ; T-1× → T-1WIRE ;
  C4 "hooks → obs sink" DROPPED (contradicts I13 deliberate exemption) ;
  invented "economy" component REMOVED (rtk/cavshrink = package provision) ;
  C3 picker is engine/steps/forms.go (host wizard), NOT tui ;
  D2 citation fixed (k8s uses *[]T pointer; we choose explicit sentinel — in-repo precedent plugins.Plan.Configured) ;
  C2 relabelled expand→CUTOVER (not ParallelChange — D4 removes the safety window) ;
  carry.go:44-46 (44=harness keep, 45=plugins/46=skills remove) ; D = container ~/.claude dotfiles via docker-exec, not bind-mount.
  GAPS closed: .env→inventory seeding (C2-step2) ; absent/malformed inventory behaviour (C2-step3) ;
  Inventory Go type + json tags + version forward-compat (§-1) ; new-INV mechanical homes (§6) ; C3 catalog call-site enumeration.

CORR (iteration 1 → 2): see prior — gh-skill not git-clone ; Desired→Inventory ; dual-wiring/empty-mask promoted to FACT ; obs 6 not 5 ; ListByExt ∄ ; watcher 2 const ; app 20 fields.

HYPO:
  reported "skills didn't install" root = empty=healthy mask ∧ carryStart ordering — mechanism is FACT;
  causal link to the symptom not reproduced in-container.

Q:
  samber/cc-skills-golang also a plugin-marketplace? Today consumed as a gh-skill source (skills.txt:1). golang→plugin reclass DEFERRED (needs evidence; would contradict #145).
  D2 sentinel vs k8s pointer: owner-signed divergence; revisit only if JSON round-trip of nil-vs-[] proves load-bearing.
```
