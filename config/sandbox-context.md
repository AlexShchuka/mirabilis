You are running isolated dev container.

- **Storage**
  - `/workspace` — Work only here; organize it however you like.
  - `~/.claude` — **persistent memory** and `~/.config`. Keep them tidy — store durable memory here, not scratch.
  - `/tmp` and everything else is **ephemeral**. Scratch only; no inbox, no drop folder — bring files in by editing `/workspace`.
- **Memory layout:**
  - Durable memory lives in `~/.claude/memory/` as category files. Each holds brief one-line
    invariants as `- ` bullets. `MEMORY.md` is auto-generated on session start — do not edit it.
  - Categories (semantic = timeless facts; procedural = how-to; episodic = dated events):
    - `about-me` (semantic) — identity, role, goals, hard preferences, constraints
    - `workstreams` (semantic) — active and recurring projects: what, where, status pointer
    - `dev-principles` (procedural) — cross-project engineering invariants: style, testing bar, anti-slop
    - `sandbox-ops` (procedural) — how to operate this container: tools, build/run commands, gotchas
    - `domain-knowledge` (semantic) — durable facts from studying, researching, reading
    - `research-log` (episodic) — dated findings from a specific investigation, paper, or bug; append-only, compact when long
    - `prep` (episodic) — interview-prep state: topics drilled, weak spots, study targets
    - `shared-codebook` (semantic) — bilaterally agreed term mappings: term → definition — example — external anchor; hard cap ~20 entries; real boundary is kernel-reducibility (Common Code v3)
  - Keep `~/.claude/rules/*.md` ONLY for genuine path-scoped per-file-type rules (the `paths:`
    frontmatter mechanism), born when a real convention is learned — not shipped empty.
