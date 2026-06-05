SHELL := /bin/bash
MODULE := github.com/aashs25/hsbillnradius
BIN_DIR := bin
BINARIES := api radius worker scheduler migrate
GO ?= go
COMPOSE ?= docker compose
COMPOSE_FILE := deploy/docker-compose.yml

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## Sync go.mod/go.sum
	$(GO) mod tidy

.PHONY: build
build: ## Build all binaries into bin/
	@mkdir -p $(BIN_DIR)
	@for b in $(BINARIES); do echo ">> building $$b"; $(GO) build -trimpath -o $(BIN_DIR)/$$b ./cmd/$$b || exit 1; done

.PHONY: test
test: ## Run all tests with the race detector
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover: ## Write an HTML coverage report to coverage.html
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: fmt
fmt: ## Format and simplify code
	gofmt -w -s .

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: run-api
run-api: ## Run the api-service
	$(GO) run ./cmd/api

.PHONY: run-radius
run-radius: ## Run the radius-service
	$(GO) run ./cmd/radius

.PHONY: run-worker
run-worker: ## Run the worker-service
	$(GO) run ./cmd/worker

.PHONY: run-scheduler
run-scheduler: ## Run the scheduler-service
	$(GO) run ./cmd/scheduler

.PHONY: migrate-up
migrate-up: ## Apply all pending DB migrations
	$(GO) run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(GO) run ./cmd/migrate down

.PHONY: migrate-status
migrate-status: ## Show migration status
	$(GO) run ./cmd/migrate status

.PHONY: sqlc
sqlc: ## Generate type-safe query code (requires sqlc)
	sqlc generate

.PHONY: infra-up
infra-up: ## Start dev infra (Postgres + Redis)
	$(COMPOSE) -f $(COMPOSE_FILE) up -d postgres redis

.PHONY: infra-down
infra-down: ## Stop and remove dev infra
	$(COMPOSE) -f $(COMPOSE_FILE) down

.PHONY: infra-logs
infra-logs: ## Tail dev infra logs
	$(COMPOSE) -f $(COMPOSE_FILE) logs -f

.PHONY: tools
tools: ## Install dev tools (goose, sqlc)
	$(GO) install github.com/pressly/goose/v3/cmd/goose@latest
	$(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

.PHONY: ci
ci: build vet test lint ## Run the full local CI pipeline
