# ADR + ТЗ: «Песочница для Claude» (`claude-sandbox`) — v4.0

> Локальный **workspace** на macOS (в духе Coder Workspaces): декларативная, воспроизводимая, изолированная среда, где **автономный** Claude развивает по Git + локальным IDE репозитории `neuro-matrix`, `mozgoslav` и **саму `claude-sandbox`**.
> *Что изменилось против v3.1:* (1) центральная модель — **workspace с разделением ФС на persistent / ephemeral** (словарь Coder), а не «контейнер вообще»; (2) `neuro-matrix` зафиксирован как **Claude Code плагин**, грузится как системный промт через `--plugin-dir`; (3) **bypass-permissions ON** переведён из «принятого риска» в **поддерживаемый путь** — официальный devcontainer Anthropic под non-root именно для этого; (4) **geo-exit = host-VPN, allowlist = firewall в контейнере** (исправлен конфликт §3↔D7 из v3.1); (5) добавлена **предустановка MCP/плагинов** как первоклассное требование; (6) D12 (Terraform/Coder) **частично переоткрыт** — литеральный Coder задокументирован как шов.

- **Статус:** Proposed · **Дата:** 2026-06-05 · **Владелец:** Sasha · **Платформа:** MacBook Air M5
- **Объект:** отдельный репозиторий `claude-sandbox`. `neuro-matrix`/`mozgoslav` — части экосистемы, не предмет ADR.
- **Принцип:** решения письменно; конкретная реализация (Dockerfile/compose/devcontainer/скрипты) — отдельной задачей после согласования.

---

## 1. Цель
Развернул на маке → получаю **workspace** (как у Coder: декларативный, воспроизводимый, изолированный) → внутри живёт **автономный** Claude с включённым **bypass-permissions**, под системным протоколом `neuro-matrix`. Развиваю **три репо** (`neuro-matrix`, `mozgoslav`, `claude-sandbox`) через **Git + локальные IDE** (Rider и др.). **Файловая система изолирована** и явно разделена на **персистентную** (память, репозитории, auth — переживают рестарт/пересборку) и **временную** (скрэтч, кэши, свежий плагин — стираются на stop). MCP-серверы и плагины **предустанавливаются** декларативно; добавить/убрать — одной командой.

## 2. Место в экосистеме
| Репо | Что это | Связь с workspace |
|---|---|---|
| **`claude-sandbox`** (этот ADR) | Локальный workspace-движок | развиваешь её **в ней же** (dogfooding) |
| **`neuro-matrix`** | **Claude Code плагин**: `CLAUDE.md` + `agents/` (developer/analyzer/critic/epistemic-auditor) + `hooks/` (cycle/approval/verification gates) + `invariants.txt`. Анти-«нейрослоп» harness. | **системный промт workspace** — грузится плагином на старте (см. D4) |
| **`mozgoslav`** | Голосовой harness «над любым LLM» | один из развиваемых репо |
| *(будущие)* | … | так же |
Связь workspace↔репо — **Git** (правишь в IDE на хосте, агент — в контейнере, один источник правды). MCP — общий механизм расширения (§4/D10), не привязан к репо.

## 3. Регион и сеть — **один egress-слой** (исправление v3.1)
Claude официально недоступен в РФ (регион не в списке [1]; IP проверяется на каждом запросе). **Факт, который v3.1 прятал:** workspace физически не работает, пока трафик Claude не выходит из поддерживаемой страны. Решение разнесено на два уровня:
1. **Геовыход = VPN на хосте** (сейчас — hidemy.name на маке). Весь трафик мака, включая Docker Desktop, выходит через VPN. **Это легально и на твоей стороне.** Отдельного tunnel-контейнера нет (KISS).
2. **Allowlist (default-deny) = firewall внутри `coder`-контейнера** (iptables/ipset, паттерн reference-devcontainer Anthropic [3]) — безопасность автономного агента; см. D7.
**Риск Mac-специфики (R9):** Docker Desktop не всегда наследует host-VPN (gVisor/vpnkit-роутинг, DNS/MTU). **Обязательная проверка на старте** (`make doctor`): из контейнера `curl https://api.anthropic.com` должен идти через VPN-exit. Если нет — fallback: WireGuard/SOCKS внутри egress-proxy (шов O2).
*Ограничение:* firewall не делает TLS-инспекцию → domain-fronting в принципе возможен; для **доверенных** репо это принятый риск (R3).

## 4. ТЗ — требования
**Функциональные (FR):**
- FR1 — автономный кодер-агент локально, управление из терминала; коннект из IDE (Coder-style attach).
- FR2 — **изоляция ФС мака**: виден только `workspace`; всё прочее на хосте недоступно агенту.
- FR3 — **ФС разделена на persistent / ephemeral** (см. §7) — явная политика, а не случайные тома.
- FR4 — контролируемый egress: единый слой allowlist + tunnel-exit (§3), default-deny.
- FR5 — **bypass-permissions ON** (автономность облачного кодера); реализуется через non-root + контейнер-границу (D3).
- FR6 — **`neuro-matrix` как системный промт**: на каждом старте/рестарте тянется **свежим с GitHub** (`AlexShchuka/neuro-matrix`, отревьюенный ref) и грузится плагином (D4); его `CLAUDE.md`/agents/hooks активны.
- FR7 — **предустановка MCP/плагинов**: декларативный набор (вшит в образ + provisioning-скрипт); `add/remove` тривиально; без потолков (D10).
- FR8 — **память в виде папок** на persistent-томе; плагин/скрэтч — свежие/чистые на старте.
- FR9 — ветки/коммиты/PR в forge (GitHub через `gh`).
- FR10 — **мультирепо workspace**: те же файлы правишь в локальной IDE; один источник правды (включая саму песочницу).
- FR11 — замкнутая петля: отревьюенные изменения плагина/конфига возвращаются на следующем старте.
- FR12 — one-click установка, отдельным репозиторием.

**Нефункциональные (NFR):**
- **NFR1 — нейтральность через KISS.** Части подменяемы за счёт **конфига + слабосвязанных тулзов + документированных швов**; абстракцию/интерфейс вводим **только при ≥2 реальных реализациях** (rule of three).
- NFR2 — простота (сложность экспоненциально усложняет отладку LLM). *Следствие для D1:* полный Coder (coderd+Postgres+provisionerd) для одного workspace — избыточен; берём его модель, не control-plane.
- NFR3 — **воспроизводимость**: пины образа, тега плагина, набора MCP **и версии самого Claude Code CLI** (поведение автономки меняется между версиями).
- NFR4 — least-privilege: **non-root** (обязательно для bypass), секреты в ENV/стор, `~/.ssh` не монтируем, OAuth-токен инжектится в рантайме (не в образ).
- NFR5 — соответствие принципам (ADR, review-gate `neuro-matrix`, без параллельных решений).
- NFR6 — наблюдаемость (логи egress, `claude mcp list`, статус forge, тайминг старта, какой ref плагина загружен).

## 5. Подменяемые швы (список мест замены, не фреймворк)
Движок workspace · egress/tunnel · агент · плагины · набор MCP · auth · forge · IDE-связка · инсталлер. Выбор — в одном конфиге/`.env`. Реальный интерфейс заводим, лишь когда появляется второй адаптер. Дефолты — §6.

## 6. Решения (дефолты)
- **D1 Движок workspace = Docker Desktop + compose** (модель Coder: декларативный шаблон, persistent/ephemeral ресурсы, IDE-attach). Граница = контейнер + сеть. Подменяемо на Colima/OrbStack/Podman. **microVM не используем** (доверенный код — §9). **Литеральный Coder** (coderd+Terraform-шаблон) — задокументированный шов на потом, если workspace’ов станет много; для одного мака — избыточен (NFR2). *Это частично переоткрывает D12 v3.1.*
- **D1a Образ минимальный** (v1, фокус «только песочница»): Node.js (для Claude Code CLI) + `git` + `gh` + GitHub MCP + firewall-тулзы (`iptables`/`ipset`). **Без рантаймов проектов** (Python/.NET/Go) — добавляются позже под конкретные репо, шов.
- **D2 Auth = личная подписка Claude.** `claude setup-token` → ~годовой OAuth-токен в `CLAUDE_CODE_OAUTH_TOKEN`, инжектится в рантайме (persistent-том/стор, не образ). Регион — §3. *Открытый вопрос O1:* подтвердить по текущему ToS допустимость unattended-автономки на личной подписке и её usage-лимиты.
- **D3 Права = bypass-permissions ON** — `--dangerously-skip-permissions`. **Поддерживаемый путь:** Claude Code отказывается под root, поэтому крутим под **non-root** юзером; контейнер = граница. Это ровно официальный devcontainer-паттерн Anthropic (firewall + non-root под bypass) [3]. Более мягкая альтернатива (шов) — `--permission-mode auto` (классификатор проверяет действия, работает и под root). Принятый риск — R3.
- **D4 `neuro-matrix` как системный промт:** на каждом старте `git clone` **с GitHub** (`AlexShchuka/neuro-matrix`, reviewed-ref) в **ephemeral**-том → грузим **плагином через `--plugin-dir`** (его `CLAUDE.md`/agents/hooks/MCP активируются автоматически, без подтверждения по каждому тулу [7]). Свежесть гарантируется ephemeral-классом тома (§7): старый плагин стирается, тянется заново. Fallback «копия CLAUDE.md» из v3.1 **удалён** (YAGNI).
- **D5 Review-gate:** с GitHub тянем **reviewed-ref** (тег/защищённая ветка, не `main`), в духе mutation-gate `neuro-matrix`. **Anti-brick:** держим `last-known-good` ref; если reviewed-ref не стартует — откат на него (новое против v3.1; защита dogfooding-петли).
- **D6 Память = папки** на **persistent**-томе: нативная память Claude Code — иерархия `CLAUDE.md` + auto-memory `~/.claude/projects/<project>/memory/MEMORY.md` [5]. Без knowledge-graph/vector (markdown надёжнее извлекается; добавим по реальной нужде).
- **D7 Egress = allowlist (default-deny) внутри контейнера; exit — host-VPN (§3).** Реализация — iptables/ipset firewall в `coder`-контейнере (паттерн reference-devcontainer Anthropic [3]): root на старте резолвит allowlist в IP и ставит default-deny, затем агент работает non-root и iptables менять не может. В allowlist минимум: Anthropic API, GitHub (`github.com`, `api.github.com`, `codeload.github.com`), `api.githubcopilot.com` (GitHub MCP), npm-реестр (`registry.npmjs.org`). Геовыход — host-VPN, отдельного tunnel-контейнера нет.
- **D8 IDE-связка = общая папка** (мультирепо bind-mount) на маке, IDE на хосте (Rider/VS Code). **Конвенция против гонок:** агент работает в своей ветке / git-worktree; одновременных правок одного файла избегаем. *Mac-нюанс:* bind-mount только в shared-путях Docker Desktop [9]; тяжёлые артефакты (node_modules/build) — в volume, не в bind-mount (virtiofs).
- **D9 Forge = GitHub** (всё: код, проекты И плагин `neuro-matrix`). `gh` для PR (в `main` без подтверждения не пушит). **GitHub MCP — предустановлен** (D10). Плагин тянется с GitHub на рестарте (D4).
- **D10 MCP/плагины = открытый, предустанавливаемый набор. День 1 — GitHub MCP.** Декларативно: provisioning-скрипт `claude mcp add ...` на старте (`registry.npmjs.org` в allowlist для stdio-серверов; чтобы `npx` не тянулся каждый раз — кэш в persistent-томе, R6). `add/remove` — одной командой. Без фиксированного числа и потолков.
- **D11 Инсталлер = Homebrew + clone.** `Brewfile` + `brew bundle` (Docker Desktop cask, `gh`, …); саму песочницу — `git clone` + `make up`. Свой `brew tap`+formula — опц. позже.
- **D12 Terraform/Coder control-plane отклонён для одного хоста** (provisioning кластера ≠ «склонировать ветку + поднять контейнер + настроить MCP»). **Но** модель Coder (persistent/ephemeral, декларативный шаблон) принята (D1); литеральный Coder — шов.

## 7. Модель файловой системы (ядро v4) — persistent vs ephemeral
Словарь Coder: ресурс **persistent** (переживает stop/restart) или **ephemeral** (уничтожается на stop). Раскладка контейнера `coder` (non-root, напр. `/home/coder`):

| Том | Класс | Что внутри | Поведение |
|---|---|---|---|
| `/workspace` | **persistent** (bind-mount в shared-путь мака) | три репо — `neuro-matrix`, `mozgoslav`, `claude-sandbox` | **один источник правды**; правишь в IDE на хосте и агентом в контейнере |
| `/home/coder/.claude` | **persistent** (named volume) | auth (`.credentials`/OAuth), **память** (`projects/<p>/memory/MEMORY.md`), настройки | переживает рестарт; FR8 |
| `mcp-data` | **persistent** (named volume) | состояние/данные предустановленных MCP | переживает рестарт |
| `/home/coder/plugins/neuro-matrix` | **ephemeral** | плагин, подтянутый **с GitHub** (reviewed-ref) | **свежий на каждом старте** (FR6/FR11) — стирается и тянется заново с GitHub |
| `/tmp`, `/home/coder/.cache`, build-каталоги | **ephemeral** (tmpfs/anon-volume) | скрэтч, кэши сборки | стираются на stop — чистая среда, воспроизводимость |
| остальной хост-мак | **невидим** | — | FR2: вне `/workspace` агент не видит ничего |

Принцип: **состояние, которое должно жить → persistent том; всё, что должно быть свежим/чистым → ephemeral.** Память и auth — persistent; плагин и скрэтч — ephemeral.

## 8. Сквозной флоу (кратко)
**Ф0 (раз):** включить host-VPN (hidemy.name) → `brew bundle` (Docker Desktop + `gh`) → `git clone claude-sandbox` → папка-`workspace` в shared-путь → конфиг (`.env`: `PLUGIN_REF`, allowlist, токены, OAuth-токен) → `make build` (CLI@pin) → `make doctor` (проверка: VPN-exit виден из контейнера, gh-auth) → `make setup-token`/логин подпиской.
**Цикл:** включил мак (VPN ON) → `make up`: старт `coder`-контейнера → entrypoint под root ставит **firewall** (allowlist, default-deny) → дропает в **non-root** `coder` → монтирует persistent (память+auth+npm-кэш), создаёт ephemeral (плагин+скрэтч), `git clone` reviewed-ref `neuro-matrix` с GitHub в ephemeral, регистрирует **GitHub MCP** → `make claude`: `claude --dangerously-skip-permissions --plugin-dir .../neuro-matrix` → агент правит код в `/workspace`, тесты, `@critic`-gate, PR через `gh` → ты в IDE правишь те же файлы → merge → двигаешь reviewed-ref → `make restart` подтягивает новый плагин (память сохранена, скрэтч чист).

## 9. Ландшафт 2026 (кратко)
- **Workspace-модель:** Coder («secure environments for developers and their agents» [Coder]) — Terraform-шаблоны, persistent/ephemeral ресурсы, Wireguard, IDE-attach. Для одного мака берём **модель и словарь**, а не control-plane (NFR2); реализуем devcontainer/compose. Anthropic ships reference **devcontainer** с firewall+non-root под bypass [3] — наш дефолтный паттерн для D3.
- **Песочница:** локальный контейнер + сетевой боундари (код свой, M5, KISS). microVM (Firecracker/E2B/Apple `container`) — для **недоверенного** кода; не наш кейс.
- **Нативный sandbox Claude Code** (Seatbelt/bubblewrap, `sandbox.*` в settings) — рассмотрен и **отклонён как основной**: даёт изоляцию, но не workspace-модель (persistent/ephemeral, IDE-attach, предустановка) и слабее по ФС-границе от остального мака. Можно включить **внутри** контейнера как defense-in-depth.
- **Нейтральность агента:** `AGENTS.md` — кросс-инструментальный файл инструкций; как в `mozgoslav` (`AGENTS.md` canonical + `CLAUDE.md` symlink).
- **MCP/память:** file/git/bash — встроенные; память надёжнее на markdown; `npx`/`uvx` на старте медленны → **вшивать в образ** (D10/R6).

## 10. Риски
| # | Риск | Митигирование |
|---|---|---|
| R1 | Claude в РФ недоступен; геодоступ вне scope | tunnel-exit встроен в egress-слой (§3/D7); легальный доступ — на твоей стороне; архитектура от региона не зависит |
| R2 | Подписочный OAuth с нестандартного/меняющегося exit-IP → флаги аккаунта | стабильный exit; мониторинг; запасной auth-путь (API-ключ) |
| R3 | bypass ON + эксфильтрация из контейнера (нет TLS-инспекции) | **принятый риск**: только доверенные репо; non-root; egress default-deny; секреты не монтируем |
| R4 | `--plugin-dir`: повторно проверить активацию hooks/MCP плагина на твоей версии CLI | по докам активируются автоматически [7]; провалидировать на пине; иначе `auto`-mode |
| R5 | bind-mount только в shared-путях мака; uid-маппинг под non-root | `workspace` в shared-путь; uid-маппинг в образе; проверить права записи |
| R6 | `npx`/`uvx` тянут MCP на старте (медленно/сеть) | **вшить MCP в образ**; реестры в allowlist как fallback |
| R7 | dogfooding: битый reviewed-tag брикует старт | `last-known-good` тег + авто-откат (D5) |
| R8 | стоимость/лимиты автономных циклов на подписке | лимиты расхода (ENV), модель попроще для рутины; O1 (ToS/лимиты) |
| R9 | Docker Desktop может не наследовать host-VPN (DNS/MTU/routing) | обязательный `make doctor`: curl до Anthropic из контейнера через VPN-exit; fallback — туннель в egress-proxy (O2) |

## 11. Решено / открытые развилки
**Решено:** workspace-модель в духе Coder (persistent/ephemeral ФС) · движок Docker Desktop + compose (microVM не нужен; полный Coder — шов) · **bypass ON через non-root + контейнер** (поддерживаемый путь) · подписка + `setup-token` · RU: geo-exit = host-VPN (hidemy.name), allowlist = firewall в контейнере · forge = GitHub (всё) · `neuro-matrix` = плагин-системный-промт с GitHub через `--plugin-dir` (свежий на рестарте) · **GitHub MCP предустановлен** · образ минимальный (без рантаймов проектов) · память-папки (persistent) · мультирепо · нейтральность через KISS · инсталлер Homebrew + clone.

**Открытые развилки (до реализации):**
- **O1** — подтвердить по текущему ToS Anthropic допустимость и usage-лимиты unattended-автономки на **личной** подписке (влияет на R8/D2).
- **O2** — geo-exit = host-VPN (hidemy.name); подтвердить `make doctor`, что Docker Desktop его наследует (R9). Fallback — WireGuard/SOCKS в egress-proxy.
- **O3** — литеральный Coder vs compose: дефолт compose; пересмотреть, если workspace’ов станет >1.

## Источники
1. Anthropic — Supported countries: `anthropic.com/supported-countries`
2. Claude Code — Authentication / `setup-token` (OAuth-токен, хранение, подписка): `code.claude.com/docs/en/authentication`
3. Claude Code — Development containers (devcontainer, non-root, firewall, bypass): `code.claude.com/docs/en/devcontainer`
4. Claude Code — Sandboxing (Seatbelt/bubblewrap, `sandbox.*`, «не полная изоляция»): `code.claude.com/docs/en/sandboxing`
5. Claude Code — Memory (CLAUDE.md иерархия + auto-memory `projects/<p>/memory`): `code.claude.com/docs/en/memory`
6. Claude Code — Permission modes (bypass vs auto; отказ под root): `code.claude.com/docs/en/permission-modes`
7. Claude Code — Plugins / `--plugin-dir` (hooks+MCP активируются): `code.claude.com/docs/en/plugins`
8. Claude Code — MCP (`claude mcp add`, scopes, npx/uvx pain): `code.claude.com/docs/en/mcp`
9. JetBrains Rider — Docker на macOS (bind-mount только в mapped-путях): `jetbrains.com/help/rider/docker.html`
10. Claude Code + GitHub (PR через `gh`): `code.claude.com/docs/en/common-workflows`
11. `neuro-matrix` — Claude Code плагин (CLAUDE.md/agents/hooks/invariants): `github.com/AlexShchuka/neuro-matrix`
12. Coder — secure environments for developers and their agents (workspace-модель, persistent/ephemeral): `github.com/coder/coder`, `coder.com/docs`
— Ландшафт песочниц/workspace 2026; Homebrew/Brewfile/Colima; AGENTS.md; YAGNI/KISS (rule of three) — по результатам ресёрча.
