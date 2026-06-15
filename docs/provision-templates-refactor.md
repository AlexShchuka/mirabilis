# Provision → Template-Driven Sandbox: Refactor Plan (C1–C4)

Status: **design doc, owner-review pending**. Encoding: English + first-order logic where a real
formal object exists; dense prose elsewhere. Per-claim tag `[FACT file:line]` · `[ASSOC]` (agent-sourced
line, re-verify at impl) · `[HYPO]` · `[Q]`. Home of locked decisions: session anchors A1–A8.

Scope law (G2): `M` (sandbox) provisions/exposes a mechanism; *when/whether* to use it is `N` (harness).
A Δ deciding behaviour is mis-placed here. Mission frame (INV-A): every Δ is justified by
⟨truth, capability, self-sufficiency, evolvability⟩ — **token/LOC economy is a consequence, never a rationale**.

---

## §0 — Ontology

```
Stages      Σ = {C1, C2, C3, C4}           ordered, bottom-up
Templates   T = {code, science, …}          |T| open; today |T_impl| = 1 (implicit "code")
Catalog     Skills, Plugins                  declared sets per template
Selection   sel : the fact "what the user wants"
Legacy bus  D = {.mirabilis-skills, .mirabilis-plugins-disabled, .mirabilis-harness}   (host→container dotfiles)
            D⁺ = D ∪ {.mirabilis-theme, .mirabilis-headroom-upstream, create/start markers}
Economy     ECON = {rtk, caveman, cav-shrink, headroom, localLLM}
DesiredState ds = ⟨Skills:[…], Plugins:[…]⟩    resolved, opt-in, extensible

resolve   : T → ds                            [LOCATION: host]
channel   : ECON → Stream                      stream ownership (organ-evolution-plan §0)
install_s : Skill  → git clone → ~/.claude/skills/<name>/
install_p : Plugin → `claude plugin install <name>@<market>`
Bootable(x), Green(x) : predicates over a repo state x (sandbox launches; tests+lint+arch pass)
Reads(step, src), Writes(actor, file), Owner(fact)
```

`[FACT docker-compose.yml:41]` host-bind-mount `./.mirabilis ↔ /workspace/.mirabilis` already exists.
`[FACT .devcontainer/Dockerfile:133,138]` `MIRABILIS_REPO=/opt/mirabilis`; container config root = `$MIRABILIS_REPO/config` = `/opt/mirabilis/config` (image-baked, ≠ bind-mount).
`[FACT provision.go:28-37]` `Deps` carries 8 fields + many helper methods; `[eval]` "god-object" + "`carry.go` ≥6 responsibilities" are judgments, not mechanical facts.

---

## §1 — Invariants (preserved by every stage)

```
INV-SAFE (A5)   ∀ pr ∈ PRs(plan) : Bootable(after(pr)) ∧ Green(after(pr))
INV-STRANGLER   ∀ migration μ : μ = ⟨expand, migrate, contract⟩
                  ∧ ∀ φ ∈ μ : Bootable(after(φ))                       (Fowler strangler-fig)
INV-FWD         ∀ i<j : after(Cᵢ) ⊑ after(Cⱼ)                          (monotone toward target; no churn-back)
INV-IDEM (I3)   ∀ step ∈ {skills, plugins} : Run(step) ⇒ Check(step)=⊤
INV-DISJOINT    ∀ ci,cj ∈ ECON, i≠j : channel(ci) ∩ channel(cj) = ∅    (PRESERVE; organ-evolution-plan §0)
INV-I1 (sec)    token ∉ content(desired.json) ∧ boundary(desired.json) = boundary(session-key)
INV-G2          resolve ∈ mechanism(M) ; "which template / when" decision ∈ N
INV-MISSION     ∀ Δ : justify(Δ) ∈ {truth,capability,self-sufficiency,evolvability} ∧ justify(Δ) ≠ economy
```

`[FACT SECURITY.md:70]` `.mirabilis/` already carries non-secret host↔container facts (chat-id);
`session-key` (0600) is the secret precedent ⇒ INV-I1 holds for `desired.json` (names only).

---

## §2 — Target architecture

```
T  --resolve(host)-->  ds  --write(host,native)-->  ./.mirabilis/desired.json
                                                          |
                                          (bind-mount, mkdir+own first)
                                                          v
                                   container reads /workspace/.mirabilis/desired.json
                                                          |
                                                 reconcile.Missing (I3)
                                                          v
                              ∀ s∈ds.Skills install_s(s) ; ∀ p∈ds.Plugins install_p(p)
```

Critic-fix (the load-bearing correction):
```
[FACT .devcontainer/Dockerfile:133,138] config_root(container) = $MIRABILIS_REPO/config = /opt/mirabilis/config  (image)  ≠  bind-mount
  ⇒ resolve MUST run on host (host reads config/templates/* live from the repo working tree)
  ⇒ container reads ONLY desired.json (resolved lists) ; ¬Reads(container_step, catalog)
  ⇒ "image-baked config" ceases to constrain template editing
```

Seam target after C2:
```
|{ f ∈ D : Reads(provision, f) }| = 0          (skills, plugins-disabled, harness migrated)
Owner(sel) = config (single)                    (no polarity inversion: ds is opt-in for both)
.mirabilis-theme, headroom-upstream, markers : OUT of seam (different mechanism)
```

---

## §3 — Stage transitions (Δ + strangler + touch-matrix)

Each stage is ONE PR. Cell = what a system does in that stage; ∅ = untouched.

### C1 — DRY-floor (`engine/fsio` leaf) + install-canon
```
Δ1  new leaf  engine/fsio  ⊇ {ReadJSON, WriteJSON(atomic), CopyFile, PathExists, ReadLines, ReadLineSet, Flock, WriteFile}
Δ2  arch-lint: register engine-fsio BEFORE switching callers           (else depguard violation) [critic]
Δ3  callers adopt fsio (thin-wrap old → switch → delete)
Δ4  install-canon: golang skill-set is a PLUGIN  [FACT github samber/cc-skills-golang]
      add `samber/cc` → marketplaces.txt ; `cc-skills-golang@samber` → plugins.txt
      drop npx/gh-skill paths ; install_s ≡ git clone only
```
| system | Δ | tag |
|---|---|---|
| provision-core | jsonio.go {readJSON,writeJSON,copyFile} (unexported today) + carry.go {readSet,readLines,exists} → fsio.{ReadJSON,WriteJSON,CopyFile,…} (exported leaf) | `[FACT jsonio.go:9-57, carry.go:115-146]` |
| economy-organ | rtk/headroom/cav-shrink adopt fsio; **localLLM ∅** | `[ASSOC rtk.go:40, cavshrink.go:40-80]` |
| notify | queue.go atomic-write/ReadJob/ListByExt → fsio (Job types stay) | `[ASSOC queue.go:38-162]` |
| daemon/cmd | flock/sessionkey/client file-prims → fsio | `[ASSOC flock_unix.go:19-35, sessionkey.go:15-36]` |
| TUI · container · security | ∅ | |
```
strangler(C1): expand(fsio added, old=wrappers) → migrate(switch callers) → contract(delete dups)
Bootable: ⊤ (pure-fn move, behaviour ≡).   net-LOC < 0.
```

### C2 — DesiredState JSON seam + dual-wiring fix
```
Δ1  host: facade.LaunchSteps → saveDesiredState → fsio.WriteJSON(repo/.mirabilis/desired.json)
      pre: mkdir+own .mirabilis  [critic A2-gap]
Δ2  apply.go desired()+Check() read desired.json   (write-path AND Check migrate TOGETHER — else Check/Run thrash) [critic]
Δ3  provision reads ds from desired.json ; KEEP harnessChoice() (harness global, A4) [critic]
Δ4  remove {pluginsStep, skillsStep} from carryStart  (dual-wiring: installed 2× today) [FACT carry.go carryStart ~41-47; Start() wrapper calls it at provision.go:48-51]
Δ5  empty-guard: not-configured ≠ configured-empty  (missing sel ⇏ silent skip)
```
| system | Δ | tag |
|---|---|---|
| daemon/cmd (host) | saveDesiredState write-path | `[ASSOC facade.go:123-125]` |
| engine/steps | apply.go desired()+Check() JSON-first→JSON-only→drop printf | `[FACT apply.go:33-87]` |
| provision-core | carry.go selected/disabled ← desired.json; harnessChoice kept; carryStart trimmed | `[FACT carry.go:107-113]` |
| container/security | bind-mount write; mkdir+own; **INV-I1 ⊤** | `[FACT compose:41, SECURITY.md:70]` |
| economy · notify · TUI | ∅ (independent) | `[FACT economy ¬Reads D]` |
```
strangler(C2): expand(read desired.json ∨ dotfile) → migrate(write+read JSON only) → contract(rm 3 dotfiles + printf)
Bootable: ⊤ via fallback window.
```

### C3 — Template layer (host-resolve) — user-visible
```
Δ1  config/templates/<name>/{skills.txt, plugins.txt}            (greenfield-additive; nothing reads it yet)
Δ2  config.ResolveTemplate(name) → ds                            [LOCATION host]
Δ3  TUI: forms.newConfig := {templateGroup(single-select), stacksGroup} ; drop {plugins,skills} multi-groups
      huh.Select already supported ; strings → tui/strings (I8) ; default(template)=code
Δ4  host saveDesiredState resolves chosen templateID (not raw SKILLS/PLUGINS)
```
| system | Δ | tag |
|---|---|---|
| config | templates/ dir + ResolveTemplate | `[FACT config.go:26-36]` |
| TUI/UX | single-select template + keep stacks; drop 2 groups; new strings | `[ASSOC forms.go:32-159, form.go:28-57]` |
| daemon/cmd | resolve templateID host-side | `[ASSOC facade.go]` |
| container | ∅ (resolve host-side; image-baked config irrelevant) | `[FACT critic]` |
| economy · notify · provision-core | ∅ | |
```
empty-guard: move flat skills.txt/plugins.txt under templates/code/ ONLY AFTER catalog-read is redirected. [critic]
strangler(C3): expand(templates/code = copy of flat; default code) → migrate(menu→template→host-resolve) → contract(rm flat + collapse multi-wizard)
Bootable: ⊤ (default code ≡ today).   greenfield-additive (absence of templates/ = the work).
```

### C4 — God-object teardown + single obs sink + dispatch
```
Δ1  provision.Deps → capability-split : Executor ⊕ FileIOer ⊕ per-step narrow needs
      ∀ step : step embeds only needs(step)            (accept-interfaces, G8)
Δ2  dissolve carry.go/provision.go builders ; dedupe headroom.go scriptOK wrapper [ASSOC headroom.go:132]
Δ3  daemon/cmd: main.run → dispatch-table ; single newDeps factory (dedupe facade vs runProvision)
Δ4  obs: 5 hook stderr-writes → obs sink ; hooks.SetObs in runTUI   (G5/I13) [ASSOC hooks/{postToolUse,telegram,session}.go]
Δ5  notify: queue.go split {types|io|gc} ; 3 constants → config (G4) [ASSOC queue.go, watcher.go:18-19]
Δ6  TUI/app god-object (21 fields): waiting/menuAction/busy → substructs (internal, behaviour ≡) [ASSOC app.go:51-72]
Δ7  tighten .go-arch-lint.yml (skills/plugins/reconcile already registered post-#145) ; drop plumbing tests
```
```
ECON capability needs (C4 split blueprint) [ASSOC]:
  rtk        ⊇ {Runner, argvOK, stream, settingsPath}
  headroom   ⊇ {Runner, ProxyAddr, headroomBin, upstreamPath, scriptOK, stream}
  cav-shrink ⊇ {Runner, Repo, claudeJSONPath, stream}
  localLLM   ⊇ {Runner, scriptOK, output, stream}
  ⇒ no cross-step import ; INV-DISJOINT preserved by construction
strangler(C4): internal refactor; external behaviour ≡; Green per step.
```

---

## §4 — Safety theorems

```
T-BOOT     ⊢ ∀ Cᵢ : Bootable(after(Cᵢ))            by strangler-fallback (C1 wrappers, C2 dotfile-fallback, C3 default=code, C4 behaviour-≡)
T-SEC      ⊢ INV-I1                                  content(desired.json) ⊆ {skill,plugin names} ; ⊉ secret ; boundary ≡ session-key
T-ECON     ⊢ INV-DISJOINT after C1..C4              economy ¬Reads D ; C1=fsio-only ; C4=narrow-adapters ; channels unchanged
T-NOREG    ⊢ ¬∃ step : (Reads(step, catalog)=∅ ⇒ Check(step)=⊤ ⇒ silent-skip)   guarded by empty-guard (C2,C3)
```

---

## §5 — Out of scope (recorded decisions; reopen needs new evidence, not association)

```
authproxy upstream = api.anthropic.com (hardcoded)        SECURITY.md — intentional token chain
localllm edge = host.docker.internal:1234                 SECURITY.md / AGENTS.md — intentional egress edge
framework G0–G12 / I1–I13 / engine-tui split / M-N split  healthy; not refactored
headroom-upstream + create/start markers                  different mechanism; NOT the seam
memory + MemoryCategories                                 global, NOT template-scoped (A4)
```

---

## §6 — Verification (per stage)

```
∀ Cᵢ : Green(after(Cᵢ)) ⟺  go test -race ./...  ∧  golangci-lint run ./...  ∧  go-arch-lint check  ∧  bats(test/)
Bootable(after(Cᵢ)) ⟺ e2e: launch ⇒ template chosen ⇒ skills∪plugins installed
                              ∧ empty-sel ⇒ explicit log (¬silent)
                              ∧ relaunch ⇒ zero-change (I9/G7)
                              ∧ economy alive (rtk gain ∧ headroom perf)
C0 (prereq): git pull --ff-only  (origin/main = #145: subpackages + grouped skills.txt + gh-skill — reconcile C1 canon against them)
```

---

## §7 — Ledger

```
FACT   docker-compose.yml:41 bind-mount ; .devcontainer/Dockerfile:133,138 image-config ;
       provision.go:28-37 Deps (8 fields) — "god-object" label is [eval] ;
       SECURITY.md:70 .mirabilis non-secret channel ; github samber/cc-skills-golang = plugin marketplace ;
       dual-wiring skills/plugins in carryStart (carry.go ~41-47) ∧ own phase ; Start() calls carryStart at provision.go:48-51.
ASSOC  tui/app/cmd/hooks/economy/notify internal line-refs (Explore agents; re-verify at impl).
HYPO   "skills didn't install" root = empty=healthy mask + dual-wiring ordering (not reproduced in-container).
Q      C0 pull may already land #145 gh-skill canon → reconcile A6 (npx vs gh-skill vs git-clone) at C1.
```
