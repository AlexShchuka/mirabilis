# mirabilis — Architecture

mirabilis is a single-command macOS dev container that runs Claude Code as an autonomous
AI coding agent inside an isolated Docker sandbox. Host orchestration and in-container
provisioning are one Go program; bash is limited to the places it is structurally required.

---

## Principles

1. Go is the only application language. Bash is limited to three structurally-required
   places (§1).
2. Encapsulation and low coupling come from small consumer-defined interfaces,
   function-valued strategies, and explicit dependency injection. There are no
   controller/service/repository layers, no dependency-injection container, and no `pkg/`
   tree.
3. Abstractions exist only where the program requires them.
4. Package boundaries suit a small codebase and extend to a larger one without
   restructuring.
5. The container is the security boundary: inside it the agent is unrestricted, and
   behavioural limits are the harness's responsibility. Egress is open.
6. All user-facing text, identifiers, and documentation are in English.

---

## 1. Language boundary

Go is the single application language: one module, one binary, dispatched into roles by
subcommand (§3). Host orchestration, container provisioning, Claude hooks, JSON handling,
git identity, and MCP registration are all Go.

Bash exists in three structurally-required places, and nowhere else:

| Where | Reason |
|---|---|
| `install.sh` | The bootstrap one-liner (`curl … \| bash`) runs before a Go toolchain or the binary exists. |
| `Dockerfile` (`RUN …`) | A container image is built by a shell; `RUN` lines are build directives. |
| `Makefile` | Build orchestration over `make`. |

The Dockerfile contains build-scoped conditionals: architecture detection for the
Go/rtk/uv downloads, checksum verification, and the `STACKS`-gated dotnet install. No
standalone shell script carries provisioning or orchestration logic. Each state predicate
— for example, whether the harness is installed — is defined once, in Go, and reused by
the pipeline, the provisioner, and the status line.

---

## 2. Repository layout

Module path `github.com/AlexShchuka/mirabilis`, `go.mod` at the repository root. The binary
is named `mirabilis`. There is no `src/` directory.

```
mirabilis/                          # repo root = Go module root
  go.mod  go.sum
  ARCHITECTURE.md  README.md  AGENTS.md  CLAUDE.md→AGENTS.md  SECURITY.md  LICENSE
  Makefile  Brewfile  install.sh    # the only standalone shell
  docker-compose.yml
  .editorconfig  .gitignore  .dockerignore

  cmd/
    mirabilis/main.go               # role dispatch: TUI | provision | hook

  internal/
    runner/                         # the Runner interface (leaf: no internal imports)
    ui/                             # lipgloss styles + strings (leaf)
    config/                         # config/* file IO + config value types (leaf)
    runtime/                        # execRunner, host, keychain, docker, composeEnv
    pipeline/                       # DAG engine + Step interface + StepMeta + RetryPolicy
    provision/                      # idempotent ensurers, state predicates, Status
    steps/                          # per-domain step impls + init() registry
      container/  claude/  harness/  auth/  preflight/
    ghauth/                         # interactive GitHub device-flow screen
    hooks/                          # Claude Code hook handlers (telegram, …)
    app/                            # host TUI: root model, screens, routing

  .devcontainer/
    Dockerfile                      # build-time only; cache-friendly layers
    devcontainer.json               # lifecycle hooks → `mirabilis provision …`

  config/                           # editable, file-per-concern, read by Go
    settings.json  plugins.txt  stacks.txt
    memory/rules/*.md  sandbox-context.md

  .build/                           # host-built linux binary for the image (gitignored)

  internal/**/*_test.go             # Go tests (fake Runner + unit)
```

Packages are grouped by domain: the engine, the steps, the TUI, the provisioner, and the
OS-facing primitives are distinct packages. `internal/` provides compiler-enforced privacy.

### Import graph

The package graph is acyclic. Arrows mean "imports":

```
runner   (leaf — interface only)
ui       (leaf)
config   (leaf)
runtime   → runner
pipeline  → runner, ui                  # Step.Check/Run take runner.Runner; renders via ui
hooks     → runtime
provision → runner, runtime, config     # ensurers, predicates, Status/ComputeStatus
steps     → pipeline, provision, runtime # impls delegate to provision; register into pipeline
ghauth    → runtime, ui
app       → pipeline, steps, provision, ghauth, runner, config, ui
cmd/mirabilis → app, provision, hooks    # role dispatch
```

Placement rules:

- `StepMeta`, `RetryPolicy`, and the `Step` interface live only in `pipeline`. `config`
  holds config-file IO and its own value types, not engine types.
- `Status` and `ComputeStatus` live in `provision`: status aggregates provisioning
  predicates (harness present? plugins enabled?) and container state, so it depends on
  `provision`'s predicates and on `runtime`. `runtime` never imports `provision`.

---

## 3. One binary, several roles

`mirabilis` is a single Go binary. The host runs it as a TUI; the container runs the same
binary, built for `linux` and baked into the image, under role subcommands.

| Invocation | Runs on | Role |
|---|---|---|
| `mirabilis` | host (macOS) | TUI launcher: menu, launch pipeline, hand-off to Claude |
| `mirabilis provision --phase create` | container | one-time provisioning (devcontainer `onCreateCommand`) |
| `mirabilis provision --phase start` | container | per-boot provisioning (`postStartCommand`) |
| `mirabilis hook <name>` | container | Claude Code hook handler (e.g. `hook telegram`) |

Host and container subcommands share `internal/provision`, `internal/runtime`, and
`internal/config`, so "is X installed?", the `settings.json` merge, and harness
installation each exist once.

### Build flow for the container binary

- `make` cross-compiles it on the host:
  `CGO_ENABLED=0 GOOS=linux GOARCH=<container arch> go build -o .build/mirabilis-linux ./cmd/mirabilis`.
  The code is pure Go (no cgo), producing one static binary (~7 MB). The binary ships as
  one artifact, including host-only TUI code; there is no build-tag split.
- `GOARCH` matches the container: `arm64` on Apple Silicon, `amd64` on Intel.
- The host launcher runs this build in the container-readiness step, before `devcontainer up`.
- The `Dockerfile` adds it as a late, cache-friendly layer:
  `COPY .build/mirabilis-linux /usr/local/bin/mirabilis`. The Go toolchain stays out of the
  image; only the finished binary is copied in.
- `.dockerignore` permits `.build/` and excludes everything else not needed by the build
  context. A source change bumps the image version (§9), which busts the binary layer.

---

## 4. Host TUI

The host TUI follows The Elm Architecture (Bubble Tea). The root model (`app`) holds an
enum for the active phase (`menu / pipeline / form / ghauth`) and routes `tea.Msg` to the
active sub-model — a "model of models".

Routing:

- Global messages (quit, global keys) are handled at the root.
- Contextual messages (keys, navigation) go to the active sub-model.
- `WindowSizeMsg` is broadcast to every sub-model.
- Background-task messages (pipeline-done, gh-done, per-step results) are handled at the
  root, the only model that sees every message while a step's background goroutine runs
  behind an interactive screen.

Long-running background work runs as a `tea.Cmd` (channel + self-rearming listen command,
or `Program.Send`); the model is never mutated from a goroutine, and each step's
`context.Context` is tied to quit so goroutines cannot emit into a stopped program.

Screens: `menu` (the action list), `pipeline` (launch progress), forms (`plugins`,
`harness`, `stacks`, `reset`, built on `huh`), and `ghauth` (device-flow, §11). Strings are
English and live in `internal/ui`. Menu actions: Launch · Plugins · Harness · Stacks · Open
in VS Code · Reset · Quit. Actions that require a running container are disabled, with a
hint, when the container is down.

---

## 5. The launch pipeline

### 5.1 Engine (`internal/pipeline`)

A DAG executor: it resolves dependencies; for each step it runs `Check` first and skips the
step if the desired state already holds, otherwise runs `Run` under the step's
`Retry`/`Timeout`. An `Optional` step that fails is skipped rather than failing the run.
`Interactive` steps suspend the flow under an interactive screen and are processed one at a
time through a queue (the registered set has one: GitHub sign-in). A progress bar,
spinner, and elapsed timer render the run. The engine owns the `Step` interface.

```go
// internal/pipeline
type Step interface {
    Check(ctx context.Context, r runner.Runner) (bool, error)
    Run(ctx context.Context, r runner.Runner) error
}

type StepMeta struct {
    Name, Title, Detail string
    Deps                []string
    Retry               RetryPolicy
    Optional            bool
    Interactive         bool
    Timeout             time.Duration
}
```

Behaviour is a two-method interface (`Check`/`Run`); metadata is data (`StepMeta`), supplied
at registration (§5.2).

### 5.2 Steps (`internal/steps/*`)

Each step is its own type in its own file, grouped by logical domain — `steps/container`
(update, container readiness), `steps/claude` (Claude config, theme), `steps/harness`,
`steps/auth` (gh sign-in), `steps/preflight` (egress to api.anthropic.com). A step
registers itself with its metadata:

```go
// internal/steps/harness/harness.go
type step struct{}

func (step) Check(ctx context.Context, r runner.Runner) (bool, error) {
    return provision.HarnessInstalled(ctx, r)
}
func (step) Run(ctx context.Context, r runner.Runner) error {
    return provision.EnsureHarness(ctx, r)
}

func init() {
    steps.Register(pipeline.StepMeta{
        Name: "harness", Title: "neuro-matrix", Detail: "installing/updating the neuro-matrix harness",
        Deps: []string{"prepare"}, Retry: pipeline.RetryNet, Optional: true, Timeout: 180 * time.Second,
    }, step{})
}
```

Registration:

- The `init()` functions run when the `steps` package is imported; a new step is one new
  file, with no central list to edit.
- The registry lives in one package. `Register` panics on a duplicate name.
- `Register` records a registration index. The engine executes in topological order over
  `Deps` and renders steps sorted by that order, with the registration index as a stable
  tiebreak, so independent steps render in a fixed order.
- Tests snapshot and reset the registry.

`Check` and `Run` delegate to shared `internal/provision` functions (`HarnessInstalled`,
`EnsureHarness`), reused by the provisioner and the status line. Shared container/docker
helpers (`containerRunning`, `isStale`, container env lookup) live in `internal/runtime`.

---

## 6. Provisioning (`internal/provision`)

Provisioning is a set of idempotent ensurers with guard semantics — they act only when the
desired state is absent: `EnsureSettings`, `EnsureHarness`, `EnsurePlugins`, `EnsureMCP`,
`EnsureGitIdentity`, `EnsureMemoryRules`, `EnsureTheme`, `EnsureSkills`, `EnsureRTK`. Each
state predicate (`HarnessInstalled`, `PluginEnabled`, …) is the single source of truth,
reused by the launch pipeline (§5) and the status line.

- `EnsureSettings` merges the seed `config/settings.json` into `~/.claude/settings.json` and
  drops `.sandbox`. The merge is recursive: nested objects (`hooks`, `statusLine`,
  `permissions`) merge key-by-key with the seed overriding leaves on conflict, and arrays
  are replaced. `provision/settings.go` implements this over `map[string]any`.
- `EnsurePlugins` installs the plugin set from `config/plugins.txt` (the catalog, including
  `claude-hud@claude-hud` for the status line) minus the user's disabled list, and writes
  `enabledPlugins` into settings. The `statusLine` command in `settings.json` targets
  claude-hud's version-globbed `dist/index.js`, so updates are followed without re-setup.
- `EnsureHarness` installs/updates the neuro-matrix harness from its marketplace and
  symlinks the resolved cache directory to `~/.neuro-matrix`. `HarnessInstalled` is the
  predicate behind it.
- `EnsureMCP` registers the fixed MCP server set idempotently (remove-then-add): `context7`
  (http), `sequential-thinking` (stdio via npx), and `arxiv` + `docling` (stdio via uvx,
  when uvx is present). The transport/arg/header table is data in Go.
- `EnsureGitIdentity` derives name/email from `gh api user`, falling back to the GitHub
  noreply address, and sets the global git identity. `EnsureMemoryRules` seeds the
  path-scoped rule files into `~/.claude/rules`. `EnsureSkills` clones the external
  `interview-coach` skill into `~/.claude/skills`, pulling if present and tolerating an
  offline network. `EnsureRTK` runs `rtk init -g --auto-patch` once, guarded on the
  PreToolUse hook's absence and bounded by a timeout.

### Lifecycle phases

| Concern | Phase | Mechanism |
|---|---|---|
| apt base, Go, Node, rtk, uv, optional dotnet stack, the `mirabilis` linux binary | build | `Dockerfile` — baked, layer-cached |
| seed `settings.json`, install plugins + harness, memory rules, register MCP, clone skills, `rtk init`, statusline (into the persistent `claude-home` volume) | create | `mirabilis provision --phase create` ← `onCreateCommand` |
| git token + identity, secrets, re-assertion of invariants | start | `mirabilis provision --phase start` ← `postStartCommand` |

The `provision` package exposes per-phase entry points (`Create`, `Start`) that run the
ensurers for their phase; a new concern is a new `EnsureX` wired into a phase.

Provisioning runs regardless of how the container is entered — TUI launch, VS Code attach,
or a bare `devcontainer exec` — because it is bound to the container lifecycle hooks. The
host launch pipeline checks state (`Check`) and, when stale, triggers the same functions.

---

## 7. Claude hooks (`internal/hooks`)

Claude Code hooks (`Stop`, `Notification` → Telegram) are Go subcommands wired in
`config/settings.json` as `mirabilis hook telegram`. The handler reads the event JSON from
stdin, tolerates empty or non-JSON input, sends the notification, and exits 0; the
`timeout` in `settings.json` bounds it. Token and chat id come from the container
environment (§8). The status line is the `claude-hud` plugin (§6); its `statusLine` command
invokes the plugin's bundled script.

---

## 8. Secrets and host→container injection

The single host→container secret path is the launcher's environment construction
(`internal/runtime`, `composeEnv` + `keychainGet`):

- The host launcher reads optional secrets from the macOS Keychain (`TELEGRAM_BOT_TOKEN`,
  `TELEGRAM_CHAT_ID`) and computes `MIRABILIS_VERSION` (host short HEAD) and `STACKS`, then
  injects them into the environment of every `devcontainer up` / `exec`.
  `docker-compose.yml` surfaces them to the container via `environment:`.
- `MIRABILIS_VERSION` and `STACKS` are dual-purpose: compose `args:` at build time (baked as
  `ENV`) and `environment:` at run time (for staleness comparison, §9).
- The `hook telegram` subcommand reads token/chat from that environment. GitHub and Claude
  sign-in use native flows persisted in sandbox volumes; no secret touches the repository.

---

## 9. Image version and staleness

The image bakes `MIRABILIS_VERSION` (the host's short HEAD at build) and `MIRABILIS_STACKS`.
`internal/runtime` (`isStale`) compares the container's baked values against the host's
short HEAD and the desired `STACKS`; on drift it rebuilds the image (remove image +
`devcontainer up`). The menu header surfaces `stale`. The linux binary (§3) is baked into
the image, so a source change — which moves HEAD — busts the binary layer through the same
version bump.

Stack selection writes `STACKS` to the gitignored `<repo>/.env`; it flows as a build-arg
into the image and participates in the `isStale` comparison, so changing stacks triggers a
rebuild. `.env` is host-side only and never committed.

---

## 10. The OS-facing seam (`internal/runner`, `internal/runtime`)

`runner.Runner` is a small consumer-defined interface — `Host(ctx, name, args…)`,
`Container(ctx, args…)`, `Repo()` — the single seam between Go and the outside world. The
concrete `execRunner` (in `runtime`) shells out to the real CLIs: `git`, `docker`,
`devcontainer`, `gh`. Tests inject a fake `Runner`.

`runtime` owns the host primitives: `composeEnv`/`keychainGet` (§8), Docker readiness
(`ensureDocker`, including the host-side preflight), container introspection
(`containerRunning`, `isStale`, env lookup), and hand-off. Hand-off launches Claude via
`docker exec -it`, which provides a PTY whose size and SIGWINCH Docker forwards.

---

## 11. GitHub sign-in (`internal/ghauth`)

`ghauth` drives `gh auth login --web` inside the container, scrapes the device code and
verification URL from the container's output, and opens that URL in the host browser
(macOS `open`). It is the launch pipeline's interactive step: the engine suspends the
pipeline under the `ghauth` screen and resumes when sign-in finishes. Step goroutines continue
behind this screen; their results are handled at the root model (§4).

---

## 12. Container definition (Docker / devcontainer)

- `Dockerfile` — build-time only: the base (`node:24-trixie-slim`), system packages, Go,
  Node tooling, rtk (with checksum verification), uv, the optional dotnet stack behind a
  `STACKS` gate, and the cross-compiled `mirabilis` binary in `/usr/local/bin`. Stable,
  expensive work sits in cache-friendly layers.
- `devcontainer.json` — lifecycle hooks point at the binary:
  `onCreateCommand: mirabilis provision --phase create`,
  `postStartCommand: mirabilis provision --phase start`. The host-side Docker preflight is
  part of the launcher (`runtime.ensureDocker`); there is no separate `initialize.sh`.
- `docker-compose.yml` — named volumes (`workspace`, `claude-home`, `gh-config`),
  `seccomp=unconfined`, `host.docker.internal`, `tmpfs /tmp`, `restart: unless-stopped`.
  `tini` is PID 1; the container runs as a long-lived process.

### Config resolution

Container subcommands read config from `/opt/mirabilis/config` (baked by `COPY config/`);
the host TUI reads from `<repo>/config`. `internal/config` takes the base directory as a
parameter. Config is baked, so an edit to `config/` reaches the container on the next
rebuild, consistent with the staleness model (§9).

---

## 13. Configuration (`config/`)

File-per-concern, read by Go:

- `settings.json` — the Claude settings seed (theme, status line, hooks).
- `plugins.txt` — the Claude Code plugin catalog (incl. `claude-hud`).
- `stacks.txt` — optional build stacks (e.g. `dotnet`; node/python/go are in the base
  image).
- `memory/rules/*.md` — path-scoped memory rules (YAML `paths:` frontmatter).
- `sandbox-context.md` — the sandbox context prepended to the agent's system prompt.

Plugin enable/disable, stack selection, and harness on/off are set from the TUI forms and
persisted as state (`<repo>/.env` and dot-files in the `claude-home` volume). Configuration
is data: plain files.

---

## 14. Security boundary

- The container is the boundary. Inside it: root, `sudo`, any file. Behavioural limits (no
  push/force to `main`, no credential exfiltration) are the harness's responsibility.
- Egress is open — the container reaches the network directly, with no in-container
  allowlist and no proxy; `WebFetch`/`WebSearch` work.
- Secrets — GitHub/Claude sign-in use native flows persisted in sandbox volumes; the only
  host-side secret is the optional Telegram token from the macOS Keychain, injected at run
  time (§8). The Docker socket is part of the secret trust boundary.
- Trust model — trusted code only; untrusted workloads require a stronger boundary (microVM)
  than a container. Details in `SECURITY.md`.

---

## 15. Distribution, build, tests

Distribution: `curl -fsSL …/install.sh | bash` clones into `~/.mirabilis`, runs
`make bootstrap` (`brew bundle` over the `Brewfile` — `docker-desktop`, `bash`, `go`,
`node` — plus `npm i -g @devcontainers/cli`), then `make install` (build the host binary,
symlink `mirabilis` onto `PATH`). The container binary is baked into the image at build
(§3). Target platform is macOS.

Build: `Makefile` targets `bootstrap`, `install`, `uninstall`, `menu` (host build), `linux`
(cross-compile the container binary into `.build/`), `up`, `down`, `clean`. `reset` tears
down container, image, and volumes — destroying `workspace`, `claude-home`, `gh-config` —
behind a confirmation.

Tests:

- Fake-`Runner` unit tests cover orchestration (step order, deps, retry, error handling) and
  provisioning logic (ensurer idempotency, the `settings.json` deep-merge, the state
  predicates, registry registration).
- Plain unit tests cover pure logic (parsing, formatting, retry backoff, message routing).
- `bats` covers the irreducible shell (`install.sh` smoke).
- CI: `gofmt -l` clean · `go vet` · `go build` · `go test -race`; a nightly canary runs a
  best-effort `docker build` + `devcontainer up` and asserts the seeded state (`settings.json`
  has no `sandbox` block).
- Changes ship with tests.
