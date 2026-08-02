SHELL := /usr/bin/env bash
COMPOSE := docker compose
MIGRATIONS_DIR := db/migrations/postgres

.DEFAULT_GOAL := infra

.PHONY: env infra infra-up infra-down infra-reset infra-ps infra-logs \
	psql valkey-cli garage migration db-gen gen build test test-race lint vuln fmt

env:
	@test -f .env || cp .env.example .env
	@chmod 600 .env

infra infra-up: env
	$(COMPOSE) up -d --wait

infra-down:
	$(COMPOSE) down

infra-reset:
	$(COMPOSE) down -v

infra-ps:
	$(COMPOSE) ps

infra-logs:
	$(COMPOSE) logs -f

psql:
	$(COMPOSE) exec postgres psql -U "$${POSTGRES_USER:-norn}" -d "$${POSTGRES_DB:-norn}"

valkey-cli:
	$(COMPOSE) exec valkey valkey-cli

garage:
	$(COMPOSE) exec garage /garage $(args)

migration:
	@test -n "$(name)" || { echo "usage: make migration name=create_users" >&2; exit 1; }
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql

db-gen:
	go tool sqlboiler psql

gen:
	go generate ./...
	cd web && corepack pnpm gen:api

build:
	go build ./...

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	go tool golangci-lint run

vuln:
	go tool govulncheck ./...

fmt:
	go fmt ./...
