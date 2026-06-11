# Security

## Threat model

mirabilis runs an autonomous agent with `--dangerously-skip-permissions`. The
container is the security boundary, and the design assumes **trusted code only**:

- The agent has **full freedom inside the container** — root, `sudo`, any file. mirabilis
  does not gate writes or commands inside; the boundary is the container plus host isolation.
- **Egress is open** — the container reaches the network directly. There is no in-container
  allowlist: exfiltration-hardening is intentionally out of scope (simplicity > security-from-
  exfiltration). Preventing credential exfiltration is the **harness's** behavioural job.
- The container runs under Docker's **default seccomp profile** with `cap_drop: ALL` and an
  explicit add-back list (`docker-compose.yml`), narrowing the kernel attack surface for
  container escape while staying within the trusted-code model. Notable primitives removed —
  by the seccomp profile: user/mount namespaces via `unshare` (gated on CAP_SYS_ADMIN, which
  is not added back), `io_uring`, `keyctl`; by dropped capabilities: device `mknod`, raw
  sockets, `chroot`.

Do not point mirabilis at untrusted code. For untrusted workloads you need a
stronger isolation boundary (microVM) than a container provides.

## Secrets

- **GitHub** sign-in uses the native flow (`gh auth login`) and persists inside the
  sandbox volume `~/.config/gh` (`gh-config`). It never touches the repository and
  survives updates and rebuilds.
- **Claude** authentication uses a 1-year OAuth token produced by running
  `claude setup-token` on the host. On macOS the token is stored in the Keychain
  under the service name `mirabilis-claude-token-token`; on Linux and WSL it is
  stored in `~/.claude/.mirabilis-claude-token` (mode `0600`). The Go launcher
  injects it as `CLAUDE_CODE_OAUTH_TOKEN` at container launch and `blockedFromContainer`
  prevents the host environment variable from being passed through by accident.
  `claude setup-token` on the host replaces in-container `/login` as the setup path.
  Note: `~/.claude/.credentials.json` produced by in-container `/login` has higher
  precedence than the env token — if that file exists in the `claude-home` volume,
  it silently wins. The provision-phase TUI warns the owner if both are present.
- The optional **Telegram** token's source depends on the host: on macOS it lives in the
  Keychain; on Linux and WSL it lives in `~/.claude/.mirabilis-telegram-token`
  (mode `0600`). The Go launcher injects it as `TELEGRAM_BOT_TOKEN` at container
  launch; Context7's MCP server runs anonymously, with no key.
- The GitHub MCP token is derived from your `gh` login (`gh auth token`); the
  GitHub MCP server receives it as a request header, so it also
  lands in the container's per-user config on the `claude-home` volume — inside
  the container, never in the repo. Anyone with access to the Docker socket (or
  host root) can read container secrets via `docker inspect mirabilis` or
  `/proc/1/environ` — including `CLAUDE_CODE_OAUTH_TOKEN`; the Docker socket is
  part of the secret trust boundary.
- **Never commit a secret.** If a token appears in a diff, a log, or a chat,
  treat it as compromised and rotate it immediately.

## Persistent-memory poisoning

mirabilis injects memory into every session via the `PostToolUseFailure` and `SessionStart`
hooks (see `internal/hooks/hooks.go`). Any content the agent reads — web pages, MCP tool
responses, fetched documents — can trigger a memory write. If that content is crafted to
insert adversarial bullets, those bullets survive across sessions and are replayed by the
same hooks.

**Accepted risk:** The container's open-egress design (see **Egress is open** in the Threat model section above)
means there is no in-container network gate. Poisoned content reaches the agent;
the memory layer has no source attribution.

**Mitigations in place:**
- The `PostToolUseFailure` hook caps injected context at 10 bullets / 2 KB per event —
  a single poisoned file cannot flood the context window.
- Memory files live on the `claude-home` volume; they are not persisted in the repository
  and do not leave the container boundary automatically.
- The behavioural half of this defence — a source-aware memory-write gate that refuses to
  persist content fetched from untrusted URLs — is harness-side (neuro-matrix `[harness/now]`
  issue). It is not implemented in this repo.

Do not point mirabilis at untrusted content sources without the harness-side gate in place.

## Docker socket (DooD)

The devcontainer uses docker-outside-of-docker: the host Docker socket is bind-mounted
into the container at `/var/run/docker-host.sock` and proxied to `/var/run/docker.sock`
by the feature's init script. This gives the in-container agent full access to the host
Docker daemon — it can start privileged containers, mount host paths, and perform any
operation that a root-equivalent user could perform on the Docker Desktop VM. This is
consistent with the declared threat model: the container is the boundary, and behavioural
limits are the harness's job. The socket mount is also what the Ryuk reaper container
(used by testcontainers-go for test-container cleanup) requires to terminate containers
after integration tests.

## Reporting

Report suspected vulnerabilities privately via a GitHub Security Advisory on
`AlexShchuka/mirabilis` rather than a public issue.
