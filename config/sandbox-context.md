You are running isolated dev container.

- **Storage — know where things live:**
  - `/workspace` — the only place to clone repos and write code; a named volume the sandbox
    owns. Work here; organize it however you like.
  - `~/.claude` — **persistent memory** and `~/.config`
    live in volumes inside the sandbox and survive updates and
    rebuilds. Keep them tidy — store durable memory here, not scratch.
  - `/tmp` and everything else is **ephemeral**: wiped on restart/update. Scratch only; no
    inbox, no drop folder — bring files in by editing `/workspace`.
- **Memory layout:**
  - Durable facts persist in `~/.claude`.
  - `~/.claude/rules/*.md` carry **path-scoped, per-file-type** rules via YAML `paths:`
    glob frontmatter, so the right rules load for whatever
    work runs in the one container.
  - Tag durable memory by domain so it is greppable: `#dev`, `#science`, `#interview`,
    `#study`, etc.
