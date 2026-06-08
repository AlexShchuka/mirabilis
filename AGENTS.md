# AGENTS.md

**mirabilis** is a personal, open-source macOS dev container that runs Claude Code with
full autonomy, the [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) harness
preinstalled, persistent memory, and a configurable egress allowlist. `CLAUDE.md` is a
symlink to this file. Setup: `README.md`. Threat model: `SECURITY.md`. Design contract
(RU): `docs/`.

## Non-goals

Not a multi-user platform, not a hosted service, not reproducible (use-latest, no version
pinning except GitHub Actions refs), not a security gate against the agent itself. KISS
beats reliability beats security-from-exfiltration (`docs/INVARIANTS.md` I8).

## Boundaries

- The **container is the security boundary** (I5). Inside it the agent has full freedom —
  root, `sudo`, any file (I2). Behavioural limits (no push/force to `main`, no credential
  exfiltration) are the **harness's** job, not the sandbox's.
- Work lives in **`/workspace`**, a named volume the sandbox owns (opened via VS Code Dev
  Containers attach, not a host folder). `~/.claude` and `~/.config/gh` are persistent
  volumes (memory, auth). `/tmp` and everything else is ephemeral.
- Egress is default-deny: container traffic rides a host proxy, and Bash is confined to
  `config/settings.json` → `sandbox.network.allowedDomains`. New hosts go there,
  deliberately. `WebFetch`/`WebSearch` go via the Anthropic API and always work.

## Layout

`config/` your editable config · `src/` the host launcher (`bin/` + `lib/*.sh` modules +
`menu/` Go TUI) · `docker/`, `.devcontainer/`, `docker-compose.yml`, `.claude-plugin/` the
container definition · `docs/` the RU design contract · `src/test/` the bats suite.

## Don't

- No comments in code or config — prose lives in `.md` only (shell, Dockerfile, Makefile,
  JSON, YAML, `.env`).
- Never commit secrets. Sign-in is native and persists in volumes; tokens live in the
  macOS Keychain (`src/token.sh`). If a token appears in a diff, stop.
- Don't push to `main`; branch and open a PR.
- Don't restate a behaviour here — change it in its owning file.

The lint/format/test toolchain (`shellcheck`, `shfmt`, `hadolint`, `checkbashisms`, `go
test`, bats — see `.pre-commit-config.yaml` and `.github/workflows/`) is the source of
mechanical rules; don't duplicate them in prose. **How** to work lives in the neuro-matrix
harness; this file says **what** the repo is.
