# Security

## Threat model

mirabilis runs an autonomous agent with `--dangerously-skip-permissions`. The
container is the security boundary, and the design assumes **trusted code only**:

- The agent has **full freedom inside the container** — root, `sudo`, any file. mirabilis
  does not gate writes or commands inside; the boundary is the container plus host isolation.
- **Egress is open** — the container reaches the network directly. There is no in-container
  allowlist: exfiltration-hardening is intentionally out of scope (simplicity > security-from-
  exfiltration). Preventing credential exfiltration is the **harness's** behavioural job.
- The container runs with `seccomp=unconfined` (`docker-compose.yml`) for unrestricted
  in-container syscalls — acceptable under the trusted-code model; no extra Linux
  capabilities are added.

Do not point mirabilis at untrusted code. For untrusted workloads you need a
stronger isolation boundary (microVM) than a container provides.

## Secrets

- **GitHub and Claude** sign-in use the native flows (`gh auth login`, Claude's
  first-run login) and persist **inside the sandbox volumes** — `~/.config/gh`
  (`gh-config`) and `~/.claude/.credentials.json` (`claude-home`, mode `0600`).
  They never touch the repository and survive updates and rebuilds.
- The **Context7** API key lives in the macOS Keychain (`src/token.sh set
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
