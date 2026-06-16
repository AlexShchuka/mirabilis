---
status: accepted
date: 2026-06-16
decision-makers: Owner
---

# Locked external boundaries: skill install, token chain, egress edges

## Context and Problem Statement

Several boundary decisions are already recorded and load-bearing for security and reproducibility. The scheme must record them as the *target* — deliberately kept — so that "unchanged" is distinguishable from "overlooked", and so a later stage cannot silently reopen them.

## Decision Drivers

- I1 security: the real Anthropic token must never enter the container.
- Reproducibility and recorded-decision discipline (reopen needs new evidence, never association).

## Considered Options

For each boundary the alternatives were considered at the original decision and rejected; this ADR ratifies the status quo as target.

## Decision Outcome

Chosen: keep, as locked target:

- **Skill install canon = `gh skill install <repo> <skill> --agent claude-code --scope user --force`** (D1, merged in #145). The git-clone path is rejected.
- **Auth chain** `claude → headroom → host auth proxy → Anthropic`: the real OAuth token stays on the host inside the auth proxy; only a per-session key reaches the container (I1). Upstream `api.anthropic.com` is hardcoded.
- **Deliberate egress edges**: `authproxy → api.anthropic.com` and `localllm → host.docker.internal:1234` (LM Studio). Both intentional (SECURITY.md); the localllm edge carries only prompt text, never the token.
- **`hooks → os.Stderr`** is the deliberate I13 exemption: hooks run as separate short-lived in-container processes with no reachable host obs sink.

### Consequences

- Good: stable, audited security posture; the scheme states these as kept, not as defects.
- Bad: none intended.
- Neutral: reopening any item requires a **superseding** ADR backed by new evidence (incident, measurement, changed reality) — never by association.

### Confirmation

`SECURITY.md`; `.golangci.yml` forbidigo `^os\.Stderr$` exclusions cover `internal/obs`, `cmd/mirabilis`, `internal/hooks`, `_test.go`; PR #145 merged (gh-skill canon, grouped `skills.txt`, subpackages).
