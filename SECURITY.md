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

**Accepted risk — lethal trifecta:** mirabilis deliberately combines the three
conditions that field guidance identifies as maximally dangerous: private data
lives inside the container boundary, the agent routinely fetches untrusted content
(web pages, MCP responses, cloned repos), and egress is open. The
[Anthropic devcontainer docs](https://code.claude.com/docs/en/devcontainer)
recommend deny-all egress as the default; [the lethal-trifecta analysis](https://simonwillison.net/2025/Jun/16/the-lethal-trifecta/)
treats the combination as the highest-risk prompt-injection scenario. mirabilis
deviates from both deliberately — open egress is a first-class feature. The
accepted blast radius is the container contents (workspace, memory volume, any
credentials visible inside). Mitigations are the behavioural harness, the container
boundary, and the Claude and Telegram tokens never entering the container.

## Secrets

- **GitHub** sign-in uses the native flow (`gh auth login`) and persists inside the
  sandbox volume `~/.config/gh` (`gh-config`). It never touches the repository and
  survives updates and rebuilds.
- **Claude** authentication uses a 1-year OAuth token produced by running
  `claude setup-token` on the host. On macOS the token is stored in the Keychain
  under the service name `mirabilis-claude`; on Linux and WSL it is stored in
  `~/.claude/.mirabilis-claude` (mode `0600`). **The OAuth token never enters the
  container** — not as an environment variable, not on the filesystem, not in
  `settings.json`, not in the image. The container receives only a per-session key
  that is useless without the live host auth proxy. The chain is: in-container claude →
  headroom (`:8787`, observability + MCP) → host auth proxy (injects the real Bearer
  from the host-side `TokenSource`) → `api.anthropic.com`. The session key is
  random per host-process start; anyone with host root can read it from the container,
  but it grants no access once the host proxy exits. `claude setup-token` on the host
  is the only setup path; in-container `/login` and `CLAUDE_CODE_OAUTH_TOKEN` injection
  are both replaced by this chain. The proxy listens on `127.0.0.1` on macOS; on Linux it
  binds the Docker bridge gateway IP when the daemon can resolve it and falls back to
  `0.0.0.0` otherwise — the 256-bit session key remains the auth gate in both cases.
  Note: `~/.claude/.credentials.json` in the `claude-home` volume has higher precedence
  than `ANTHROPIC_AUTH_TOKEN` — if that file existed it would silently win. The provision
  `start` phase therefore hard-removes it on every launch (`claude-credentials` step, a
  non-optional gate): in-container `/login` state never survives a relaunch, so the proxy
  chain stays the only auth path.
- **Keychain write invariant (macOS):** `KeychainStore` feeds the secret via stdin
  (`cmd.Stdin = ...`), not as a `-w <value>` argument, so the token never appears in
  argv and is not visible in `ps`. This is the single keychain-write seam; callers must
  not scatter keychain writes elsewhere.
- The optional **Telegram** token's source depends on the host: on macOS it lives in the
  Keychain; on Linux and WSL it lives in `~/.claude/.mirabilis-telegram` (mode `0600`).
  The host-side notify adapter injects it into outbound requests; it never enters the
  container. Telegram **chat-id** is written to `.mirabilis/chat-id` (a bind-mounted file)
  and read per-event by the notify adapter; it is not an environment variable.
  The outbox queue is **at-most-once on failure**: `processOneJob` makes one send attempt;
  on success or failure it writes a `.status` file and `PendingJobs` never re-queues that
  job. The only duplicate path is a watcher crash after send but before `WriteStatus`
  completes — the job stays pending and is re-sent on the next start.
- The GitHub MCP token is derived from your `gh` login (`gh auth token`); the
  GitHub MCP server receives it as a request header, so it also lands in the container's
  per-user config on the `claude-home` volume — inside the container, never in the repo.
  Anyone with access to the Docker socket (or host root) can read container secrets via
  `docker inspect mirabilis` or `/proc/1/environ` — the Docker socket is part of the
  secret trust boundary.
- **Never commit a secret.** If a token appears in a diff, a log, or a chat,
  treat it as compromised and rotate it immediately.

## VS Code attach

VS Code attaches to the running container via the `attached-container` URI scheme. The
in-container claude instance reached via VS Code attach uses the same session-keyed auth
chain (headroom → host auth proxy → Anthropic), so the OAuth token remains on the host
in that context as well.

## Docker socket (opt-in)

The docker socket is **absent by default** — `docker-compose.yml` does not bind-mount it.
Enabling it requires explicitly passing the compose override:

```
docker compose -f docker-compose.yml -f compose.sock.yml up -d --build
```

`compose.sock.yml` adds `/var/run/docker.sock` to the container. When the socket is
present the in-container agent has full access to the host Docker daemon — it can start
privileged containers, mount host paths, and perform any operation that a root-equivalent
user could perform on the Docker Desktop VM. The sandbox fingerprint changes when the
socket override is applied, so re-creating the container is required. The TUI and
`SECURITY.md` surface this consequence.

## Persistent-memory poisoning

mirabilis injects memory into every session via the `PostToolUseFailure` and `SessionStart`
hooks (see `internal/hooks/hooks.go`). Any content the agent reads — web pages, MCP tool
responses, fetched documents — can trigger a memory write. If that content is crafted to
insert adversarial bullets, those bullets survive across sessions and are replayed by the
same hooks.

**Accepted risk:** The container's open-egress design means there is no in-container network
gate. Poisoned content reaches the agent; the memory layer has no source attribution.

**Mitigations in place:**
- The `PostToolUseFailure` hook caps injected context at 10 bullets / 2 KB per event —
  a single poisoned file cannot flood the context window.
- Memory files live on the `claude-home` volume; they are not persisted in the repository
  and do not leave the container boundary automatically.
- The behavioural half of this defence — a source-aware memory-write gate that refuses to
  persist content fetched from untrusted URLs — is harness-side (neuro-matrix `[harness/now]`
  issue). It is not implemented in this repo.

Do not point mirabilis at untrusted content sources without the harness-side gate in place.

## Reporting

Report suspected vulnerabilities privately via a GitHub Security Advisory on
`AlexShchuka/mirabilis` rather than a public issue.
