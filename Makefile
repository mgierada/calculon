# Settings come from .env when present; anything here can be overridden on the
# command line, e.g. `make run USER_NAME=alice`.
# USER_NAME rather than USER, which every shell already sets.
-include .env

BIN     := bin/calculon
DB_PATH ?= ./calculon.db
PKG     := ./cmd/calculon

.DEFAULT_GOAL := help

.PHONY: help build run serve test cover lint fmt import user reset-db clean

help: ## list targets
	@grep -hE '^[a-z-]+:.*## ' $(firstword $(MAKEFILE_LIST)) | awk -F ':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'

build: ## build the binary into bin/
	go build -o $(BIN) $(PKG)

run: build ## open the dashboards fullscreen (USER_NAME=alice picks a user)
	$(BIN) ui $(if $(USER_NAME),--user $(USER_NAME))

serve: build ## serve the dashboards over ssh on SSH_ADDR
	$(BIN) serve

test: ## run every test
	go test ./...

cover: ## run tests with a coverage summary
	go test -cover ./...

vet: ## check formatting and vet
	@unformatted=$$(gofmt -l cmd internal); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go vet ./...

format: ## format the code
	gofmt -w cmd internal

import: build ## import statements: make import DIR=~/Downloads/export [USER_NAME=alice]
	@test -n "$(DIR)" || (echo "usage: make import DIR=path/to/statements"; exit 1)
	$(BIN) import $(if $(USER_NAME),--user $(USER_NAME)) $(DIR)

user: build ## create a user: make user NAME=alice KEY=~/.ssh/id_ed25519.pub
	@test -n "$(NAME)" || (echo "usage: make user NAME=alice [KEY=~/.ssh/id_ed25519.pub]"; exit 1)
	$(BIN) user add $(NAME) $(if $(KEY),--key $(KEY))

reset-db: ## delete the database at DB_PATH (asks first)
	@printf "delete %s and everything imported into it? [y/N] " "$(DB_PATH)"; read answer; \
	[ "$$answer" = y ] && rm -f "$(DB_PATH)" "$(DB_PATH)-wal" "$(DB_PATH)-shm" && echo deleted || echo kept

clean: ## remove build output
	rm -rf bin
