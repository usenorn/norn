SHELL := /usr/bin/env bash
COMPOSE := docker compose
MIGRATIONS_DIR := db/migrations/postgres

.DEFAULT_GOAL := infra

.PHONY: env infra infra-up infra-down infra-reset infra-ps infra-logs garage-cors \
	psql valkey-cli garage migration db-gen gen wire build test test-race lint vuln fmt

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

garage-cors:
	AWS_ACCESS_KEY_ID="$${GARAGE_DEFAULT_ACCESS_KEY}" \
	AWS_SECRET_ACCESS_KEY="$${GARAGE_DEFAULT_SECRET_KEY}" \
	AWS_DEFAULT_REGION="$${NORN_STORAGE_REGION:-garage}" \
	aws --endpoint-url "$${NORN_STORAGE_ENDPOINT:-http://127.0.0.1:3900}" s3api put-bucket-cors \
		--bucket "$${GARAGE_DEFAULT_BUCKET:-norn-local}" \
		--cors-configuration file://deploy/local/garage-cors.json

migration:
	@test -n "$(name)" || { echo "usage: make migration name=create_users" >&2; exit 1; }
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql

db-gen:
	go tool sqlboiler psql

gen:
	go generate ./...
	cd web && corepack pnpm gen:api

wire:
	go tool wire gen ./internal

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
