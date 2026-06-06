# mirabilis — Критический аудит репозитория (исходный)

> Индекс и легенда [ВЛАДЕЛЕЦ]/[ИИ]: [README.md](README.md).
> **Провенанс: [аудит]** — машинный мультиагентный прогон по репозиторию, привязка к коду по анкерам
> `file:line` (на коммите `13ca73e`). Не диктовалось владельцем и не свободная генерация — находки
> заземлены в коде. Переосмысления через вижн помечены **[ИИ]**.

## Объём прогона

Мультиагентный аудит: **39 находок поднято, 38 подтверждено** после adversarial-проверки (1 отклонена
на этом шаге), по 5 измерениям (security 7/7, reproducibility 8/8, reliability 8/8, architecture 8/8,
ux-docs 8/7). Здесь детализированы **18 ключевых** (5 HIGH + 8 MEDIUM + 5 LOW); прочие подтверждённые
— минорные, не раскрыты. Раздел «Что НЕ является проблемой» — **отдельный список**: распространённые
опасения, которые аудит рассмотрел и отклонил как не-проблемы (в рамках задекларированной модели), он
**не входит** в счёт 38 подтверждённых. Полный сырой отчёт — вне репозитория (эфемерный вывод задачи).

## Краткий вердикт

Репозиторий функционально связен для своей ниши, но имел три системные проблемы: непоследовательный
пиннинг (иллюзорная воспроизводимость), split-orchestrator (devcontainer CLI vs Docker auto-restart
ломает per-start setup), fail-open на критическом пути (`|| true` маскирует ошибки).

## Переосмысление через вижн [ИИ]

«Противоречивые цели» из аудита при разборе с владельцем оказались конфликтами *реализации с целями*
(цели согласованы — [DECISIONS.md](DECISIONS.md) §2). Приоритеты сместились: воспроизводимость сборки —
**не цель** (I8), поэтому чисто-repro находки (H5, M6) понижены.

## HIGH

- **H1 — harness fail-open без neuro-matrix.** `bin/mirabilis:229-237`; `refresh.sh` `|| true`. Агент
  стартует с полными правами без ворот. → fail-fast (I7).
- **H2 — два оркестратора, один путь настройки.** `docker-compose.yml:44` vs `devcontainer.json:16`.
  Авто-рестарт поднимает контейнер без setup. → setup в entrypoint ([ARCHITECTURE.md](ARCHITECTURE.md) §3).
- **H3 — RTK и neuro-matrix с плавающих источников.** `Dockerfile:25`; `marketplace.json` без ref.
  → пиннинг харнеса ради надёжности (не repro).
- **H4 — `ensure_claude` советует `mirabilis restart`, пропускающий credential setup.**
  `bin/mirabilis:226,306`. → единый флоу (I1).
- **H5 — base image без дайджеста.** `Dockerfile:1`. → чисто repro; понижено (I8).

## MEDIUM

- **M1** `protect-critical.sh` — advisory hook (фейковый маркер). → честный гейт ([CONSENT.md](CONSENT.md)).
- **M2** `restart` без `ensure_proxy` — тихая потеря egress.
- **M3** preflight за `$fresh`-гейтом — не детектит сломанный egress/harness при warm attach. → F3.
- **M4** `sandbox-context.md` велит менять файл, недоступный агенту и перезаписываемый. → зоны.
- **M5** `@devcontainers/cli` без пиннинга.
- **M6** непоследовательный пиннинг. → repro, понижено (I8).
- **M7** `DISABLE_AUTOUPDATER` в 4 местах (нарушение single-ownership).
- **M8** безусловный `git fetch` + двойной rebuild-промпт на старте.

## LOW

- **L1** дубль glob-логики вместо симлинка `~/.neuro-matrix`.
- **L2** `stop_proxy` не в `rebuild_image` — dangling tinyproxy.
- **L3** мёртвый semver `VERSION="0.2.0"`.
- **L4** README quick-start ломается.
- **L5** два списка защищённых путей — риск drift.

## Что НЕ является проблемой (refuted)

`seccomp=unconfined`, токен в env, tinyproxy без Filter, `overrideCommand:false`+`sleep infinity`,
proxy env container-wide, `marketplace.json` description, apt без version-pin — приняты SECURITY.md /
самосогласованы в рамках модели «trusted code, container = boundary».

## Связь с роадмапом [ИИ]

P1 «Согласованность» ([VISION.md](VISION.md) §6) закрывает H1/H2/H4/M1/M2/M3/M7. Пиннинг харнеса (H3)
— ради надёжности. Чисто-repro (H5/M6) — по желанию (не цель).
