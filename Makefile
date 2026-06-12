SHELL := /bin/bash
.DEFAULT_GOAL := help
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
PREFIX ?= $(shell brew --prefix 2>/dev/null || echo /usr/local)
BINDIR ?= $(PREFIX)/bin
else
BINDIR ?= $(HOME)/.local/bin
endif
BIN := bin/mirabilis
LINUX_BIN := .build/mirabilis-linux
GOARCH_LINUX := $(if $(filter arm64 aarch64,$(shell uname -m)),arm64,amd64)
export MIRABILIS_VERSION := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
-include .env
export STACKS

.PHONY: help bootstrap menu linux install uninstall up down clean reset

help:
	@printf 'mirabilis — daily use is the `mirabilis` command (after `make install`)\n\n'
	@printf '  bootstrap   install Docker Desktop + host claude CLI + Go (macOS only)\n'
	@printf '  install     build the menu binary + put the mirabilis command on PATH\n'
	@printf '  menu        build the host binary to bin/mirabilis\n'
	@printf '  linux       cross-compile Linux binary to .build/mirabilis-linux\n'
	@printf '  uninstall   remove the mirabilis command from PATH\n'
	@printf '  up          build + start the workspace container\n'
	@printf '  down        stop the workspace (state kept)\n'
	@printf '  clean       remove container + image (volumes kept)\n'
	@printf '  reset       remove container + image + volumes\n'

bootstrap:
	@test "$(UNAME_S)" = Darwin || { printf 'mirabilis: bootstrap is macOS-only (Homebrew); on Linux install prerequisites via install.sh or your package manager\n' >&2; exit 1; }
	brew bundle --file=Brewfile
	git config core.hooksPath .githooks

menu:
	@mkdir -p bin
	go build -mod=readonly -ldflags "-X main.version=$(MIRABILIS_VERSION)" -o $(BIN) ./cmd/mirabilis

linux:
	@mkdir -p .build
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH_LINUX) go build -ldflags "-X main.version=$(MIRABILIS_VERSION)" -o $(LINUX_BIN) ./cmd/mirabilis

install: menu
	@mkdir -p $(BINDIR) 2>/dev/null || true
	@test -w "$(BINDIR)" || test -w "$(dir $(BINDIR))" || { printf 'mirabilis: %s is not writable — retry with a writable PATH dir, e.g. "make install BINDIR=$$HOME/.local/bin", or run "sudo make install"\n' "$(BINDIR)" >&2; exit 1; }
	@mkdir -p $(BINDIR)
	@ln -sf "$(CURDIR)/$(BIN)" $(BINDIR)/mirabilis
	@printf 'installed %s/mirabilis — run: mirabilis\n' "$(BINDIR)"
	@case ":$(PATH):" in *":$(BINDIR):"*) ;; *) printf 'note: %s is not on your PATH — add it, e.g. echo '"'"'export PATH="%s:$$PATH"'"'"' >> ~/.bashrc\n' "$(BINDIR)" "$(BINDIR)" >&2 ;; esac

uninstall:
	@rm -f $(BINDIR)/mirabilis $(BIN) && printf 'removed %s/mirabilis\n' "$(BINDIR)"

up: linux
	docker compose up -d --build

down:
	docker compose -p mirabilis -f docker-compose.yml down

clean:
	docker compose -p mirabilis -f docker-compose.yml down --rmi local || true

PRESERVE ?= 1

reset:
ifeq ($(PRESERVE),0)
	docker compose -p mirabilis -f docker-compose.yml down --rmi local -v || true
else
	@docker cp mirabilis:/home/node/.claude/memory .mirabilis/saved-memory 2>/dev/null || true
	docker compose -p mirabilis -f docker-compose.yml down --rmi local -v || true
endif
