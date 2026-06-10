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

- **GitHub and Claude** sign-in use the native flows (`gh auth login`, Claude's
  first-run login) and persist **inside the sandbox volumes** — `~/.config/gh`
  (`gh-config`) and `~/.claude/.credentials.json` (`claude-home`, mode `0600`).
  They never touch the repository and survive updates and rebuilds.
- The optional **Telegram** token lives in the macOS Keychain and is injected as
  an environment variable at run time (read by the Go launcher); Context7's MCP
  server runs anonymously, with no key.
- The GitHub MCP token is derived from your `gh` login (`gh auth token`); the
  GitHub MCP server receives it as a request header, so it also
  lands in the container's per-user config on the `claude-home` volume — inside
  the container, never in the repo. Anyone with access to the Docker socket (or
  host root) can read container secrets via `docker inspect mirabilis` or
  `/proc/1/environ`; the Docker socket is part of the secret trust boundary.
- **Never commit a secret.** If a token appears in a diff, a log, or a chat,
  treat it as compromised and rotate it immediately.

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
