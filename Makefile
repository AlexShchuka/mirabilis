SHELL := /bin/bash
.DEFAULT_GOAL := help
DC := ./src/dc.sh
PREFIX ?= $(shell brew --prefix 2>/dev/null || echo /usr/local)
BINDIR ?= $(PREFIX)/bin
MENU_SRC := src/menu
MENU_BIN := src/menu/bin/mirabilis-menu

.PHONY: help bootstrap install uninstall menu up down clean

help:
	@printf 'mirabilis — daily use is the `mirabilis` command (after `make install`)\n\n'
	@printf '  bootstrap   install Docker Desktop + the devcontainer CLI\n'
	@printf '  install     build the menu binary + put the mirabilis command on PATH\n'
	@printf '  uninstall   remove the mirabilis command from PATH\n'
	@printf '  up          build + start the workspace container\n'
	@printf '  down        stop the workspace (state kept)\n'
	@printf '  clean       remove container + image (volumes kept)\n'

bootstrap:
	brew bundle --file=Brewfile
	npm install -g @devcontainers/cli

menu:
	@mkdir -p $(dir $(MENU_BIN))
	cd $(MENU_SRC) && go build -mod=readonly -o bin/mirabilis-menu .

install: menu
	@test -w "$(BINDIR)" || test -w "$(dir $(BINDIR))" || { printf 'mirabilis: %s is not writable — retry with a writable PATH dir, e.g. "make install PREFIX=$$(brew --prefix)", or run "sudo make install"\n' "$(BINDIR)" >&2; exit 1; }
	@mkdir -p $(BINDIR)
	@printf '#!/usr/bin/env bash\nexec "%s/src/bin/mirabilis" "$$@"\n' "$(CURDIR)" > $(BINDIR)/mirabilis
	@chmod 0755 $(BINDIR)/mirabilis
	@printf 'installed %s/mirabilis — run: mirabilis\n' "$(BINDIR)"

uninstall:
	@rm -f $(BINDIR)/mirabilis && printf 'removed %s/mirabilis\n' "$(BINDIR)"

up:
	@$(DC) up --workspace-folder .

down:
	@. ./src/env.sh && docker compose -p mirabilis -f docker-compose.yml down

clean:
	@. ./src/env.sh && docker compose -p mirabilis -f docker-compose.yml down --rmi local || true
