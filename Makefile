SHELL := /bin/bash
.DEFAULT_GOAL := help
DC := ./scripts/dc.sh
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: help bootstrap install uninstall up down doctor clean

help:
	@printf 'mirabilis — daily use is the `mirabilis` command (after `make install`)\n\n'
	@printf '  bootstrap   install Docker Desktop + the devcontainer CLI\n'
	@printf '  install     put the mirabilis command on PATH\n'
	@printf '  uninstall   remove the mirabilis command from PATH\n'
	@printf '  up          build + start the workspace container\n'
	@printf '  down        stop the workspace (state kept)\n'
	@printf '  doctor      diagnose docker / tokens / sandbox / plugin / MCP\n'
	@printf '  clean       remove container + image (volumes kept)\n'

bootstrap:
	brew bundle --file=Brewfile
	npm install -g @devcontainers/cli

install:
	@mkdir -p $(BINDIR)
	@printf '#!/usr/bin/env bash\nexec "%s/bin/mirabilis" "$$@"\n' "$(CURDIR)" > $(BINDIR)/mirabilis
	@chmod 0755 $(BINDIR)/mirabilis
	@printf 'installed %s/mirabilis — run: mirabilis\n' "$(BINDIR)"

uninstall:
	@rm -f $(BINDIR)/mirabilis && printf 'removed %s/mirabilis\n' "$(BINDIR)"

up:
	@$(DC) up --workspace-folder .

down:
	@WORKSPACE_DIR="$${WORKSPACE_DIR:-$$HOME/mirabilis-workspace}" docker compose -p mirabilis -f docker-compose.yml down

doctor:
	@./scripts/doctor.sh

clean:
	@WORKSPACE_DIR="$${WORKSPACE_DIR:-$$HOME/mirabilis-workspace}" docker compose -p mirabilis -f docker-compose.yml down --rmi local || true
