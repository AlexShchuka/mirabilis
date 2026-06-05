# Security

## Threat model

mirabilis runs an autonomous agent with `--dangerously-skip-permissions`. The
container is the security boundary, and the design assumes **trusted code only**:

- The agent runs as a **non-root** user; the container confines command execution.
- Egress is **default-deny** with a tight allowlist (`docker/init-firewall.sh`).
- The geo-exit is the **host VPN**; the in-container firewall is defence-in-depth.
- The firewall does **not** perform TLS inspection, so domain-fronting to an
  allowlisted host is theoretically possible — acceptable for trusted repos only.

Do not point mirabilis at untrusted code. For untrusted workloads you need a
stronger isolation boundary (microVM) than a container provides.

## Secrets

- The **macOS Keychain is the single source of truth** for tokens
  (`scripts/token.sh set gh|claude`).
- Tokens are injected as **environment variables at run time** and are never
  written to the repository. `.env`, `secrets/`, and token files are gitignored.
- The GitHub MCP token is resolved from the environment at start; the Claude
  OAuth token authenticates inference only.
- Because tokens are delivered as container **environment variables**, anyone
  with access to the Docker socket (or host root) can read them via
  `docker inspect mirabilis` or `/proc/1/environ`. The Docker socket is part of
  the secret trust boundary — do not expose it to untrusted users or containers.
- **Never commit a secret.** If a token appears in a diff, a log, or a chat,
  treat it as compromised and rotate it immediately
  (https://github.com/settings/tokens for GitHub).

## Reporting

Report suspected vulnerabilities privately via a GitHub Security Advisory on
`AlexShchuka/mirabilis` rather than a public issue.
