# Security

## Threat model

mirabilis runs an autonomous agent with `--dangerously-skip-permissions`. The
container is the security boundary, and the design assumes **trusted code only**:

- The agent runs as a **non-root** user; the container confines command execution.
- The agent's Bash egress is a **default-deny allowlist** via Claude Code's native
  sandbox (`sandbox.network.allowedDomains`) — configurable, no iptables, no
  elevated capabilities.
- The native sandbox needs both **bubblewrap** and **socat** installed in the
  image (`docker/Dockerfile`) — bubblewrap for filesystem/process isolation,
  socat for the egress allowlist; with `sandbox.failIfUnavailable: true` a missing
  dependency makes Claude refuse to start. Bubblewrap must create user namespaces.
  Docker blocks that by default, so the container runs with `seccomp=unconfined`
  (`docker-compose.yml`). This relaxes the **outer** container's syscall
  filtering — acceptable under the trusted-code model above, since the inner
  sandbox still enforces the egress allowlist and filesystem confinement, and no
  Linux capabilities are added. For untrusted code, pair it with a stronger
  boundary (microVM).
- The sandbox confines spawned Bash commands (the main exfiltration vector). The
  Claude process and MCP traffic go to known hosts but are not themselves
  allowlist-confined. The sandbox does **not** perform TLS inspection, so
  domain-fronting to an allowlisted host is theoretically possible — acceptable
  for trusted repos only.

Do not point mirabilis at untrusted code. For untrusted workloads you need a
stronger isolation boundary (microVM) than a container provides.

## Secrets

- **GitHub and Claude** sign-in use the native flows (`gh auth login`, Claude's
  first-run login) and persist **inside the sandbox volumes** — `~/.config/gh`
  (`gh-config`) and `~/.claude/.credentials.json` (`claude-home`, mode `0600`).
  They never touch the repository and survive `mirabilis update`.
- The **Context7** API key lives in the macOS Keychain (`scripts/token.sh set
  context7`) and is injected as an environment variable at run time.
- The GitHub MCP token is derived from your `gh` login (`gh auth token`); the
  GitHub and Context7 MCP servers receive it as a request header, so it also
  lands in the container's per-user config on the `claude-home` volume — inside
  the container, never in the repo. Anyone with access to the Docker socket (or
  host root) can read container secrets via `docker inspect mirabilis` or
  `/proc/1/environ`; the Docker socket is part of the secret trust boundary.
- **Never commit a secret.** If a token appears in a diff, a log, or a chat,
  treat it as compromised and rotate it immediately.

## Reporting

Report suspected vulnerabilities privately via a GitHub Security Advisory on
`AlexShchuka/mirabilis` rather than a public issue.
