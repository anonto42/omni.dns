# ─── Metadata ──────────────────────────────────────────────────────────
SHELL := /bin/bash
BACKEND  := backend
FRONTEND := frontend/apps/web-app

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-24s$(RESET) %s\n", $$1, $$2}'

# ── Development ────────────────────────────────────────────────────────────────
.PHONY: dev
dev: ## Run backend (Air) + frontend (Vite) in parallel
	@echo "$(YELLOW)Starting dev mode...$(RESET)"
	@$(MAKE) -j2 dev-backend dev-frontend

.PHONY: dev-backend
dev-backend: ## Run backend with Air hot-reload
	@echo "$(GREEN)Starting backend (Air)...$(RESET)"
	cd $(BACKEND) && air

.PHONY: dev-frontend
dev-frontend: ## Run frontend with Vite HMR
	@echo "$(GREEN)Starting frontend (Vite)...$(RESET)"
	cd $(FRONTEND) && npm run dev

# ── Build ──────────────────────────────────────────────────────────────────────
.PHONY: build
build: build-backend build-frontend ## Build all production artifacts

.PHONY: build-backend
build-backend: ## Build Go binary (dev, no embed)
	cd $(BACKEND) && go build -o bin/dns-server ./cmd/dns-server

.PHONY: build-frontend
build-frontend: ## Build React production bundle
	cd $(FRONTEND) && npm run build

.PHONY: build-prod
build-prod: ## Build single binary with embedded frontend
	@echo "$(YELLOW)Building frontend...$(RESET)"
	cd $(FRONTEND) && npm run build
	@echo "$(YELLOW)Copying dist to $(BACKEND)/static...$(RESET)"
	rm -rf $(BACKEND)/static
	cp -r $(FRONTEND)/dist $(BACKEND)/static
	@echo "$(YELLOW)Compiling Go binary with embedded frontend...$(RESET)"
	cd $(BACKEND) && go build -tags embed -o bin/dns-server ./cmd/dns-server
	@echo "$(GREEN)Build complete: $(BACKEND)/bin/dns-server$(RESET)"

# ── Code Generation ────────────────────────────────────────────────────────────
.PHONY: generate
generate: ## Generate TypeScript types from Go API (swag → openapi → api-types.ts)
	@echo "$(YELLOW)Generating Swagger docs...$(RESET)"
	cd $(BACKEND) && $(shell go env GOPATH)/bin/swag init \
		-g cmd/dns-server/main.go -o ./docs --parseInternal
	@echo "$(YELLOW)Converting to OpenAPI 3...$(RESET)"
	cd $(BACKEND)/docs && npx -y swagger2openapi swagger.json --outfile openapi.json
	@echo "$(YELLOW)Generating TypeScript types...$(RESET)"
	npx -y openapi-typescript $(BACKEND)/docs/openapi.json \
		-o $(FRONTEND)/src/features/api-types.ts
	@echo "$(GREEN)Type generation complete.$(RESET)"

# ── Testing ────────────────────────────────────────────────────────────────────
.PHONY: test
test: test-backend test-frontend ## Run all tests

.PHONY: test-backend
test-backend: ## Run Go tests
	cd $(BACKEND) && go test ./... -v

.PHONY: test-frontend
test-frontend: ## Run Vitest
	cd $(FRONTEND) && npm run test

# ── Linting ────────────────────────────────────────────────────────────────────
.PHONY: lint
lint: lint-backend lint-frontend ## Run all linters

.PHONY: lint-backend
lint-backend: ## Run golangci-lint
	cd $(BACKEND) && golangci-lint run ./...

.PHONY: lint-frontend
lint-frontend: ## Run ESLint
	cd $(FRONTEND) && npm run lint

# ── Docker ─────────────────────────────────────────────────────────────────────
.PHONY: docker-up
docker-up: ## Start production stack
	docker compose up -d --build

.PHONY: docker-up-dev
docker-up-dev: ## Start dev stack with hot-reload volumes
	docker compose -f docker-compose.dev.yml up --build

.PHONY: docker-down
docker-down: ## Stop all containers
	docker compose down

# ── Setup ──────────────────────────────────────────────────────────────────────
.PHONY: setup
setup: ## Install all dev tools and dependencies
	@echo "$(YELLOW)Installing Go tools...$(RESET)"
	go install github.com/air-verse/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "$(YELLOW)Installing frontend dependencies...$(RESET)"
	cd $(FRONTEND) && npm install
	@echo "$(YELLOW)Installing pre-commit...$(RESET)"
	@command -v pip >/dev/null 2>&1 && pip install pre-commit --break-system-packages 2>/dev/null || echo "Please install pre-commit manually: https://pre-commit.com/#install"
	@command -v pre-commit >/dev/null 2>&1 && pre-commit install || echo "pre-commit not found, skipping install"
	@echo "$(GREEN)Setup complete.$(RESET)"

# ── Cleanup ────────────────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove all build artifacts
	rm -rf $(BACKEND)/bin $(BACKEND)/tmp $(BACKEND)/static $(BACKEND)/docs
	rm -rf $(FRONTEND)/dist
