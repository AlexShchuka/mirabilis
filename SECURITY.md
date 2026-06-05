# Security

## Threat model

mirabilis runs an autonomous agent with `--dangerously-skip-permissions`. The
container is the security boundary, and the design assumes **trusted code only**:

- The agent runs as a **non-root** user; the container confines command execution.
- The agent's Bash egress is a **default-deny allowlist** via Claude Code's native
  sandbox (`sandbox.network.allowedDomains`) — configurable, no iptables, no
  elevated capabilities.
- The native sandbox uses **bubblewrap**, which must create user namespaces.
  Docker blocks that by default, so the container runs with `seccomp=unconfined`
  (`docker-compose.yml`). This relaxes the **outer** container's syscall
  filtering — acceptable under the trusted-code model above, since the inner
  sandbox still enforces the egress allowlist and filesystem confinement, and no
  Linux capabilities are added. For untrusted code, pair it with a stronger
  boundary (microVM).
- The geo-exit is the **host VPN**.
- The sandbox confines spawned Bash commands (the main exfiltration vector). The
  Claude process and MCP traffic go to known hosts but are not themselves
  allowlist-confined. The sandbox does **not** perform TLS inspection, so
  domain-fronting to an allowlisted host is theoretically possible — acceptable
  for trusted repos only.

Do not point mirabilis at untrusted code. For untrusted workloads you need a
stronger isolation boundary (microVM) than a container provides.

## Secrets

- The **macOS Keychain is the single source of truth** for tokens
  (`scripts/token.sh set gh|claude|context7`).
- Tokens are injected as **environment variables at run time** and are never
  written to the repository. `.env` and token files are gitignored.
- Because tokens are delivered as container **environment variables**, anyone
  with access to the Docker socket (or host root) can read them via
  `docker inspect mirabilis` or `/proc/1/environ`. The Docker socket is part of
  the secret trust boundary — do not expose it to untrusted users or containers.
- The GitHub and Context7 MCP servers receive their tokens as resolved request
  headers, so those values also persist in the container's per-user config
  (`~/.claude.json`) on the `claude-home` volume — inside the container, never in
  the repo. Treat that volume as part of the secret trust boundary.
- **Never commit a secret.** If a token appears in a diff, a log, or a chat,
  treat it as compromised and rotate it immediately
  (https://github.com/settings/tokens for GitHub).

## Reporting

Report suspected vulnerabilities privately via a GitHub Security Advisory on
`AlexShchuka/mirabilis` rather than a public issue.
