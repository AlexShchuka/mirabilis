workspace "mirabilis" "mirabilis is a habitat for the agent: it builds and tends the dev-container home in which Claude Code lives, nested in the developer host and the open external world; the system is the frame, the agent is the brain (ADR-0009). Target architecture as a C4 model (Context, Container, Component, Code) of the desired state. One multi-mode Go binary plus the dev-container it provisions; the host vs in-container split is a deployment concern, modeled as deployment nodes. Conformance enforced by go-arch-lint; decisions recorded as ADRs under ./adr." {

    !identifiers hierarchical

    model {

        user = person "Owner / Developer" "Single-owner operator who launches the sandbox and runs Claude Code with autonomy." "person"

        claudecode = softwareSystem "Claude Code" "The inhabitant and brain of the home: the agent mirabilis exists to house and run; executes inside the provisioned dev-container and reaches Anthropic through the host auth proxy." "external"
        docker = softwareSystem "Docker Engine" "Host container runtime: builds the image, runs the dev-container, streams events." "external"
        anthropic = softwareSystem "Anthropic API" "api.anthropic.com — Claude model + Claude Code backend." "external"
        telegram = softwareSystem "Telegram Bot API" "Outbound notifications + channel chat-id detection." "external"
        lmstudio = softwareSystem "LM Studio (host)" "host.docker.internal:1234 — host-local OpenAI-compatible model for prompt offload." "external"
        github = softwareSystem "GitHub" "gh auth + git remote; harness/skill/plugin sources." "external"

        mirabilis = softwareSystem "mirabilis" "The habitat for the agent: builds and tends the dev-container home for Claude Code with the neuro-matrix harness, persistent memory and open egress; the frame within which the agent (the brain) works." {

            !adrs adr

            cli = container "mirabilis CLI" "A single multi-mode Go binary. The same artifact runs on the host (TUI, serve daemon) and inside the dev-container (provision, hook, localllm); see the deployment view for the planes. Components are grouped by hexagonal role." "Go" "cli" {

                # ---- Domain core (no dependency on adapters; depends on nothing external) ----
                group "Domain Core" {
                    pipeline = component "Pipeline FSM" "Command contract (Meta/Check/Run) + ordered executor: deps, optional cascade-skip, retry, per-step timeout, streaming events, Resume/cancel. Idempotency contract Check->Run->Check (I3)." "internal/engine/pipeline" "core"
                    bus = component "Bus" "NodeID addressing algebra + message/event value types for the TUI (app and leaves); the engine emits its own event types and never imports bus (depguard engine-no-tui). Carries an obs.Snapshot in one event." "internal/bus" "core"
                    provModel = component "Provisioning Model [TARGET]" "TARGET package (not yet in the tree): the desired-state of a provisioned sandbox as step contracts the use-cases compose into a pipeline. Pure domain." "internal/engine/provision/model" "core,target"
                    inventory = component "Inventory Contract [TARGET]" "TARGET package (not yet in the tree): resolved opt-in selection {version, configured, skills, plugins} with an explicit configured-sentinel; host writes inventory.json, container reconciles." "internal/engine/inventory" "core,target"
                }

                # ---- Ports: interface types (not packages); the only legal use-case<->adapter coupling ----
                group "Ports" {
                    runnerPort = component "Runner (port)" "Subprocess execution contract; every external command crosses it (I4). Interface in internal/engine/exec." "exec.Runner" "port"
                    storePort = component "Store (port)" "Secret get/set contract; one backend per platform (I7). Interface in internal/engine/secrets." "secrets.Store" "port"
                    dockerPort = component "Docker (port)" "compose up/build/down/reset + inspect/events + fingerprint. Interface in internal/engine/sandbox." "sandbox.Docker" "port"
                    notifierPort = component "Notifier (port)" "Outbound message-send contract. Interface in internal/engine/notify." "notify.Notifier" "port"
                    completerPort = component "Completer (port)" "Prompt-completion contract for host-local model offload. Interface in internal/engine/localllm." "localllm.Completer" "port"
                    tokenSourcePort = component "TokenSource (port)" "Anthropic token provider, consumed by the auth proxy. Interface in internal/engine/authproxy." "authproxy.TokenSource" "port"
                }

                # ---- Application services (use-cases): compose pipelines from core + ports ----
                group "Application Services" {
                    steps = component "Launch use-case" "Composes the host launch pipeline: preflight, claude-auth, config wizard, telegram, image, container, provision(create/start) via docker exec, gh-auth, plugins/skills apply, harness, attach." "internal/engine/steps" "useCase"
                    provision = component "Provision use-case" "Composes the idempotent in-container provisioning pipeline per phase (create | start | plugins | skills) from the model + installers." "internal/engine/provision" "useCase"
                }

                # ---- Driven adapters: implement a port and/or talk to the outside ----
                group "Driven Adapters" {
                    exec = component "Exec" "Runner implementations: Host (streamed, pgid-killed), TTY/PTYTee (interactive attach), Fake (tests)." "internal/engine/exec" "driven"
                    obs = component "Obs sink" "Single observability destination: slog file + thread-safe node-status registry with watcher fan-out (G5). Concrete struct, injected; not an interface." "internal/obs" "driven"
                    config = component "Config" "Reads tunables, catalogs and .env selections; the only writer of .env keys (G4)." "internal/engine/config" "driven"
                    harness = component "Harness data" "neuro-matrix install actions, probe/relink scripts, provision markers, start-marker hash." "internal/engine/harness" "driven"
                    reconcile = component "Reconcile" "Generic install-missing fold shared by the installers." "internal/engine/provision/reconcile" "driven"
                    secrets = component "Secrets" "Store implementation: macOS Keychain / Linux-WSL 0600 file, one-time legacy migration (I7)." "internal/engine/secrets" "driven"
                    claudeauth = component "Claude auth" "TokenSource implementation: cached host token + setup-token PTY extractor + background persistence." "internal/engine/claudeauth" "driven"
                    authproxy = component "Auth proxy" "Reverse proxy for api.anthropic.com on a host goroutine; consumes TokenSource, exposes only a per-session key to the container (I1). Upstream hardcoded (SECURITY.md)." "internal/engine/authproxy" "driven"
                    sandboxAdapter = component "Sandbox" "Docker implementation over the Moby SDK + compose: up/build/down/reset, inspect/events, fingerprint (sock changes fingerprint, I10), attach argv." "internal/engine/sandbox" "driven"
                    status = component "Status watcher" "Owns the docker-events resubscribe loop for the session; maps container health into obs; degrades, never blocks the menu (I12)." "internal/engine/status" "driven"
                    membackup = component "Memory backup" "Syncs the in-container memory dir to the host repo via docker cp. Exposes a Save function, not a port interface." "internal/engine/membackup" "driven"
                    notify = component "Notify" "Notifier implementation: Telegram sender + file-backed outbox queue with retry and delivered-pruning." "internal/engine/notify" "driven"
                    localllm = component "Local-LLM offload" "Completer implementation: OpenAI-compatible POST to the host LM Studio; also the local-offload MCP server. Deliberate egress edge (SECURITY.md). No in-project import." "internal/engine/localllm" "driven"
                    skills = component "Skill installer" "Installs Claude skills via gh skill install (canon, D1); reconciles against gh skill list." "internal/engine/provision/skills" "driven"
                    plugins = component "Plugin installer" "marketplace add + claude plugin install; writes enabled plugins to settings; carries the Plan.Configured sentinel." "internal/engine/provision/plugins" "driven"
                }

                # ---- Driving adapters: initiate work on the core ----
                group "Driving Adapters" {
                    tui = component "TUI app" "Bubble Tea root model: frame+router, engine-event<->bus bridge, tea.Exec handoff. UI thread does no I/O outside Cmd ctors (I2)." "internal/tui/app" "driving"
                    tuiLeaves = component "TUI leaves" "Rendering and dispatch only: screens, components, frame, router, strings (English, I8), styles, a11y. Never import internal/engine (§4.2)." "internal/tui/{screens,components,frame,router,strings,styles,a11y}" "driving"
                    cmd = component "CLI dispatch + composition root" "Parses subcommands (default TUI | serve | provision | hook | notify | localllm) and wires every adapter into the use-case Deps. The single composition root." "cmd/mirabilis" "driving"
                    hooks = component "Hooks" "Dispatches mirabilis hook <name> (session-start, telegram, post-tool-use-failure) as short-lived in-container processes; writes os.Stderr by deliberate I13 exemption." "internal/hooks" "driving"
                }
            }
        }

        # ---- Context (L1) ----
        user -> mirabilis.cli.tui "Launches, configures, attaches"
        mirabilis -> claudecode "Provisions the sandbox and launches"
        claudecode -> mirabilis.cli.authproxy "Claude traffic via per-session key" "HTTPS"
        mirabilis.cli.authproxy -> anthropic "Proxies, injects real token" "HTTPS"
        mirabilis.cli.notify -> telegram "Sends notifications; detects chat-id" "HTTPS"
        mirabilis.cli.sandboxAdapter -> docker "compose/build/run; inspect; events" "Docker SDK"
        mirabilis.cli.cmd -> github "gh auth / git remote" "HTTPS"
        mirabilis.cli.localllm -> lmstudio "Offloads prompts" "HTTP :1234"

        # ---- Component (L3): driving -> use-case / driven ----
        mirabilis.cli.tui -> mirabilis.cli.steps "Runs the launch pipeline"
        mirabilis.cli.tui -> mirabilis.cli.tuiLeaves "Renders via"
        mirabilis.cli.tui -> mirabilis.cli.bus "Addressed envelopes"
        mirabilis.cli.tui -> mirabilis.cli.pipeline "Consumes pipeline events"
        mirabilis.cli.cmd -> mirabilis.cli.steps "Builds Deps; dispatches (host modes)"
        mirabilis.cli.cmd -> mirabilis.cli.provision "Dispatches (provision mode)"
        mirabilis.cli.cmd -> mirabilis.cli.authproxy "Starts (serve daemon)"
        mirabilis.cli.cmd -> mirabilis.cli.notify "Starts outbox watcher (serve)"
        mirabilis.cli.cmd -> mirabilis.cli.status "Starts container watcher"
        mirabilis.cli.cmd -> mirabilis.cli.membackup "Save memory"
        mirabilis.cli.cmd -> mirabilis.cli.localllm "Serves offload MCP"
        mirabilis.cli.hooks -> mirabilis.cli.provision "Invokes on session-start"

        # ---- use-case -> domain core + ports ----
        mirabilis.cli.steps -> mirabilis.cli.pipeline "Composes & runs"
        mirabilis.cli.provision -> mirabilis.cli.pipeline "Composes & runs"
        mirabilis.cli.steps -> mirabilis.cli.provModel "Realizes desired state [target]"
        mirabilis.cli.provision -> mirabilis.cli.provModel "Realizes desired state [target]"
        mirabilis.cli.cmd -> mirabilis.cli.inventory "Resolves & writes inventory.json [target]"
        mirabilis.cli.provision -> mirabilis.cli.inventory "Reconciles against inventory.json [target]"
        mirabilis.cli.steps -> mirabilis.cli.runnerPort "uses"
        mirabilis.cli.steps -> mirabilis.cli.dockerPort "uses"
        mirabilis.cli.steps -> mirabilis.cli.tokenSourcePort "uses"
        mirabilis.cli.steps -> mirabilis.cli.storePort "uses"
        mirabilis.cli.steps -> mirabilis.cli.notifierPort "uses"
        mirabilis.cli.provision -> mirabilis.cli.runnerPort "uses"

        # ---- driven adapters implement ports ----
        mirabilis.cli.exec -> mirabilis.cli.runnerPort "implements"
        mirabilis.cli.secrets -> mirabilis.cli.storePort "implements"
        mirabilis.cli.claudeauth -> mirabilis.cli.tokenSourcePort "implements"
        mirabilis.cli.sandboxAdapter -> mirabilis.cli.dockerPort "implements"
        mirabilis.cli.notify -> mirabilis.cli.notifierPort "implements"
        mirabilis.cli.localllm -> mirabilis.cli.completerPort "implements"

        # ---- adapter / cross dependencies (real imports) ----
        mirabilis.cli.claudeauth -> mirabilis.cli.secrets "Reads cached token"
        mirabilis.cli.authproxy -> mirabilis.cli.tokenSourcePort "Consumes per request"
        mirabilis.cli.status -> mirabilis.cli.sandboxAdapter "inspect/events"
        mirabilis.cli.membackup -> mirabilis.cli.runnerPort "docker cp"
        mirabilis.cli.steps -> mirabilis.cli.sandboxAdapter "build/up/exec"
        mirabilis.cli.steps -> mirabilis.cli.provision "docker exec provision (cross-process)"
        mirabilis.cli.skills -> mirabilis.cli.reconcile "install-missing"
        mirabilis.cli.plugins -> mirabilis.cli.reconcile "install-missing"
        mirabilis.cli.provision -> mirabilis.cli.skills "skills phase"
        mirabilis.cli.provision -> mirabilis.cli.plugins "plugins phase"
        mirabilis.cli.provision -> mirabilis.cli.harness "install actions/markers"
        mirabilis.cli.tuiLeaves -> mirabilis.cli.bus "Addressed delivery"

        # ---- observability: obs is the single sink; cmd creates it, every component logs via it (per-component edges elided for clarity, see README section 4) ----
        mirabilis.cli.cmd -> mirabilis.cli.obs "Creates the sink; all components log and report status via it"

        # ---- Deployment: the host vs in-container PLANES (one binary, two run locations) ----
        deploymentEnvironment "Runtime" {
            host = deploymentNode "Developer Host" "macOS / Linux / WSL" {
                controlPlane = deploymentNode "mirabilis — control plane" "TUI + single-instance serve daemon (auth proxy, notify watcher, client reaper)" {
                    containerInstance mirabilis.cli
                }
                deploymentNode "LM Studio" "host-local model server" {
                    softwareSystemInstance lmstudio
                }
            }
            devhost = deploymentNode "Dev Container (Sandbox)" "Provisioned Docker dev-container; the security boundary; owns /workspace and the persistent ~/.claude volume" "Docker / docker-compose" {
                provisionPlane = deploymentNode "mirabilis — provision plane" "provision / hook / localllm modes (in-container processes)" {
                    containerInstance mirabilis.cli
                }
                deploymentNode "Claude Code" "the agent under autonomy" {
                    softwareSystemInstance claudecode
                }
            }
        }
    }

    views {

        systemContext mirabilis "L1_Context" "The owner, Claude Code, and the external systems mirabilis integrates." {
            include *
            autoLayout lr
        }

        container mirabilis "L2_Containers" "mirabilis is a single binary (plus the dev-container it provisions, shown in the deployment view)." {
            include *
            autoLayout
        }

        component mirabilis.cli "L3_Components" "All components of the binary, grouped by hexagonal role: core, ports, application services, driven and driving adapters." {
            include *
            autoLayout
        }

        dynamic mirabilis.cli "Launch_Flow" "Target launch sequence (single-container scope)." {
            user -> mirabilis.cli.tui "Selects Launch"
            mirabilis.cli.tui -> mirabilis.cli.steps "Runs launch pipeline"
            mirabilis.cli.steps -> mirabilis.cli.sandboxAdapter "Builds image, starts container"
            mirabilis.cli.steps -> mirabilis.cli.provision "docker exec provision (cross-process)"
            mirabilis.cli.provision -> mirabilis.cli.inventory "Reconciles selection [target]"
            autoLayout
        }

        deployment mirabilis "Runtime" "Deployment_Planes" "Where the single binary instance runs: control plane on the host, provision plane inside the dev-container." {
            include *
            autoLayout
        }

        styles {
            element "person" {
                shape Person
                background #1f4e79
                color #ffffff
            }
            element "external" {
                background #6b6b6b
                color #ffffff
            }
            element "cli" {
                background #2d6a9f
                color #ffffff
            }
            element "core" {
                shape RoundedBox
                background #c0392b
                color #ffffff
            }
            element "port" {
                shape Hexagon
                background #7d3c98
                color #ffffff
            }
            element "useCase" {
                shape RoundedBox
                background #117864
                color #ffffff
            }
            element "driven" {
                shape Component
                background #2e86c1
                color #ffffff
            }
            element "driving" {
                shape Component
                background #1e8449
                color #ffffff
            }
            element "target" {
                border Dashed
                opacity 60
            }
        }
    }
}
