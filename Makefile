# ─────────────────────────────────────────────────────────────────────────────
# OmniDNS DNS — Makefile
#
# Everything runs inside Docker. The only requirement on the host machine is:
#   docker >= 24  +  docker compose >= v2
#
# Quick start:
#   make setup     # one-time: wire git hooks + build check images
#   make dev       # start backend + frontend with hot-reload
#   make test      # lint + unit tests (same checks as pre-commit)
#   make build     # build production Docker image
#   make up        # run production image
# ─────────────────────────────────────────────────────────────────────────────
SHELL := /bin/bash

ifneq (,$(wildcard .env))
include .env
export
endif

ENV_FILE_ARG := $(if $(wildcard .env),--env-file .env,)
PROD_DNS_ADDR := $(or $(OMNIDNS_DNS_ADDR),your host LAN IP)
PROD_DNS_PORT := $(or $(OMNIDNS_DNS_PORT),53)
PROD_HTTP_PORT := $(or $(OMNIDNS_HTTP_PORT),8080)

DC_PROD  := docker compose $(ENV_FILE_ARG) -f docker/docker-compose.yml
DC_DEV   := docker compose -f docker/docker-compose.dev.yml
DC_TEST  := docker compose -f docker/docker-compose.test.yml

GREEN  := $(shell tput -Txterm setaf 2 2>/dev/null)
YELLOW := $(shell tput -Txterm setaf 3 2>/dev/null)
CYAN   := $(shell tput -Txterm setaf 6 2>/dev/null)
RESET  := $(shell tput -Txterm sgr0  2>/dev/null)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-22s$(RESET) %s\n", $$1, $$2}'

# ── One-time setup ────────────────────────────────────────────────────────────
.PHONY: setup
setup: ## Wire git hooks and pre-build check images (run once after clone)
	@echo "$(YELLOW)Registering git hooks (.githooks/)...$(RESET)"
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "$(YELLOW)Pre-building Docker check images (speeds up first commit)...$(RESET)"
	$(DC_TEST) build
	@echo "$(GREEN)Done. You only need Docker to work on this project.$(RESET)"

# ── Development ───────────────────────────────────────────────────────────────
.PHONY: dev
dev: ## Start backend (Air hot-reload) + frontend (Vite HMR) in Docker
	@echo "$(YELLOW)Starting dev stack...$(RESET)"
	@echo "  Backend  → http://localhost:8080"
	@echo "  Frontend → http://localhost:5173"
	@echo "  DNS      → localhost:5353 (UDP)"
	$(DC_DEV) up --build

.PHONY: dev-down
dev-down: ## Stop the dev stack
	$(DC_DEV) down

.PHONY: logs
logs: ## Tail logs from the dev stack
	$(DC_DEV) logs -f

# ── Testing & linting (all inside Docker) ─────────────────────────────────────
.PHONY: test
test: test-backend test-frontend ## Run all checks (lint + unit tests)

.PHONY: lint
lint: test ## Run the repository checks used before opening a PR

.PHONY: test-backend
test-backend: ## Run Go lint (golangci-lint) + unit tests in Docker
	@echo "$(YELLOW)Running Go checks...$(RESET)"
	$(DC_TEST) run --rm --no-deps backend-check

.PHONY: test-frontend
test-frontend: ## Run frontend type-checks and unit tests in Docker
	@echo "$(YELLOW)Running TypeScript check...$(RESET)"
	$(DC_TEST) run --rm --no-deps frontend-check

.PHONY: test-images
test-images: ## Rebuild the check images (needed after Go/Node version bumps)
	$(DC_TEST) build --no-cache

# ── Production build & run ───────────────────────────────────────────────────
.PHONY: build
build: ## Build the production Docker image (single binary, embedded frontend)
	@echo "$(YELLOW)Building production image...$(RESET)"
	$(DC_PROD) build
	@echo "$(GREEN)Image built: dns-server:latest$(RESET)"

.PHONY: up
up: ## Build production image and start the stack (detached)
	@echo "$(YELLOW)Building production image...$(RESET)"
	$(DC_PROD) build
	@echo "$(YELLOW)Starting production stack...$(RESET)"
	$(DC_PROD) up -d --force-recreate
	@echo "$(GREEN)Running. Dashboard → http://localhost:$(PROD_HTTP_PORT)$(RESET)"
	@echo "$(GREEN)DNS      → $(PROD_DNS_ADDR):$(PROD_DNS_PORT) (UDP)$(RESET)"

.PHONY: down
down: ## Stop all running stacks (prod + dev)
	$(DC_PROD) down        2>/dev/null || true
	$(DC_DEV)  down        2>/dev/null || true

.PHONY: restart
restart: down up ## Stop, rebuild, and restart the production stack

# ── Code generation ───────────────────────────────────────────────────────────
.PHONY: generate
generate: ## Regenerate TypeScript API types from Go swagger annotations
	@echo "$(YELLOW)Generating API types (runs inside Docker)...$(RESET)"
	$(DC_TEST) run --rm --no-deps backend-check \
		sh -c "go run github.com/swaggo/swag/cmd/swag@latest init \
		           -g cmd/dns-server/main.go -o ./docs --parseInternal && \
		       npx -y swagger2openapi docs/swagger.json --outfile docs/openapi.json"
	docker run --rm \
		-v "$(PWD)/backend/docs:/in" \
		-v "$(PWD)/frontend/src/features:/out" \
		node:22-alpine \
		sh -c "npx -y openapi-typescript /in/openapi.json -o /out/api-types.ts"
	@echo "$(GREEN)api-types.ts updated.$(RESET)"

# ── Cleanup ───────────────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove local build artefacts (not Docker images/volumes)
	rm -rf backend/bin backend/tmp backend/static backend/docs
	rm -rf frontend/dist

.PHONY: clean-all
clean-all: clean ## Remove build artefacts AND all Docker images/volumes for this project
	$(DC_PROD) down -v --rmi all 2>/dev/null || true
	$(DC_DEV)  down -v --rmi all 2>/dev/null || true
	$(DC_TEST) down -v --rmi all 2>/dev/null || true
