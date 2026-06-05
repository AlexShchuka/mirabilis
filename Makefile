SHELL := /bin/bash
.DEFAULT_GOAL := help
COMPOSE := ./scripts/compose.sh
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: help bootstrap install uninstall build up down restart ps logs doctor \
        claude agent shell token-gh token-claude token-context7 setup-token \
        check-tokens clean

help:
	@printf 'mirabilis — daily use is the `mirabilis` command (make install)\n\n'
	@printf '  bootstrap      install Docker Desktop via Homebrew\n'
	@printf '  install        put the mirabilis command on PATH\n'
	@printf '  uninstall      remove the mirabilis command from PATH\n'
	@printf '  build          build the image (pinned Claude Code CLI)\n'
	@printf '  up             start the workspace (injects Keychain tokens)\n'
	@printf '  down           stop the workspace (state kept)\n'
	@printf '  restart        recreate (re-applies firewall, refreshes plugin)\n'
	@printf '  doctor         diagnose docker/tokens/VPN/claude/plugin/MCP\n'
	@printf '  claude         launch the autonomous agent (neuro-matrix)\n'
	@printf '  agent P="..."  headless one-shot prompt\n'
	@printf '  shell          shell into the workspace as coder\n'
	@printf '  token-gh|token-claude|token-context7  store a secret in the Keychain\n'
	@printf '  setup-token    generate a 1-year Claude OAuth token in-container\n'
	@printf '  check-tokens   show which secrets are configured\n'
	@printf '  clean          remove container + image (volumes kept)\n'

bootstrap:
	brew bundle --file=Brewfile

install:
	@mkdir -p $(BINDIR)
	@printf '#!/usr/bin/env bash\nexec "%s/bin/mirabilis" "$$@"\n' "$(CURDIR)" > $(BINDIR)/mirabilis
	@chmod 0755 $(BINDIR)/mirabilis
	@printf 'installed %s/mirabilis — run: mirabilis\n' "$(BINDIR)"
	@printf 'zsh completion: add to ~/.zshrc before compinit -> fpath+=(%s/completions)\n' "$(CURDIR)"

uninstall:
	@rm -f $(BINDIR)/mirabilis && printf 'removed %s/mirabilis\n' "$(BINDIR)"

build:
	@$(COMPOSE) build

up:
	@$(COMPOSE) up -d
	@printf 'up. next: make doctor   then   make claude\n'

down:
	@$(COMPOSE) down

restart:
	@$(COMPOSE) up -d --force-recreate
	@printf 'restarted (memory/auth kept, scratch fresh)\n'

ps:
	@$(COMPOSE) ps

logs:
	@$(COMPOSE) logs -f --tail=120

doctor:
	@./scripts/doctor.sh

claude:
	@$(COMPOSE) exec -it --user coder -w /workspace mirabilis claude --dangerously-skip-permissions

agent:
	@$(COMPOSE) exec -T --user coder -w /workspace mirabilis claude -p "$(P)" --dangerously-skip-permissions

shell:
	@$(COMPOSE) exec -it --user coder -w /workspace mirabilis bash

token-gh:
	@./scripts/token.sh set gh

token-claude:
	@./scripts/token.sh set claude

token-context7:
	@./scripts/token.sh set context7

setup-token:
	@$(COMPOSE) exec -it --user coder mirabilis claude setup-token
	@printf 'copy the printed token, then run: make token-claude\n'

check-tokens:
	@./scripts/token.sh check

clean:
	@$(COMPOSE) down --rmi local || true
