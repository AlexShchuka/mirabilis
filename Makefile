SHELL := /bin/bash
.DEFAULT_GOAL := help
PREFIX ?= $(shell brew --prefix 2>/dev/null || echo /usr/local)
BINDIR ?= $(PREFIX)/bin
BIN := bin/mirabilis
LINUX_BIN := .build/mirabilis-linux
GOARCH_LINUX := $(shell case "$$(uname -m)" in arm64|aarch64) echo arm64;; *) echo amd64;; esac)
export MIRABILIS_VERSION := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
-include .env
export STACKS

.PHONY: help bootstrap menu linux install uninstall up down clean reset

help:
	@printf 'mirabilis — daily use is the `mirabilis` command (after `make install`)\n\n'
	@printf '  bootstrap   install Docker Desktop + the devcontainer CLI\n'
	@printf '  install     build the menu binary + put the mirabilis command on PATH\n'
	@printf '  menu        build the host binary to bin/mirabilis\n'
	@printf '  linux       cross-compile Linux binary to .build/mirabilis-linux\n'
	@printf '  uninstall   remove the mirabilis command from PATH\n'
	@printf '  up          build + start the workspace container\n'
	@printf '  down        stop the workspace (state kept)\n'
	@printf '  clean       remove container + image (volumes kept)\n'
	@printf '  reset       remove container + image + volumes\n'

bootstrap:
	brew bundle --file=Brewfile
	npm install -g @devcontainers/cli

menu:
	@mkdir -p bin
	go build -mod=readonly -o $(BIN) ./cmd/mirabilis

linux:
	@mkdir -p .build
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH_LINUX) go build -o $(LINUX_BIN) ./cmd/mirabilis

install: menu
	@test -w "$(BINDIR)" || test -w "$(dir $(BINDIR))" || { printf 'mirabilis: %s is not writable — retry with a writable PATH dir, e.g. "make install PREFIX=$$(brew --prefix)", or run "sudo make install"\n' "$(BINDIR)" >&2; exit 1; }
	@mkdir -p $(BINDIR)
	@ln -sf "$(CURDIR)/$(BIN)" $(BINDIR)/mirabilis
	@printf 'installed %s/mirabilis — run: mirabilis\n' "$(BINDIR)"

uninstall:
	@rm -f $(BINDIR)/mirabilis && printf 'removed %s/mirabilis\n' "$(BINDIR)"

up: linux
	devcontainer up --workspace-folder .

down:
	docker compose -p mirabilis -f docker-compose.yml down

clean:
	docker compose -p mirabilis -f docker-compose.yml down --rmi local || true

reset:
	docker compose -p mirabilis -f docker-compose.yml down --rmi local -v || true
