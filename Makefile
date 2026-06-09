# ─── Metadata ──────────────────────────────────────────────────────────
SHELL := /bin/bash
BACKEND  := backend
FRONTEND := frontend

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
	cd $(BACKEND) && go run github.com/swaggo/swag/cmd/swag@latest init \
		-g cmd/dns-server/main.go -o ./docs --parseInternal
	@echo "$(YELLOW)Converting to OpenAPI 3...$(RESET)"
	cd $(BACKEND)/docs && npx -y swagger2openapi swagger.json --outfile openapi.json
	@echo "$(YELLOW)Generating TypeScript types...$(RESET)"
	npx -y openapi-typescript $(BACKEND)/docs/openapi.json \
		-o $(FRONTEND)/src/features/api-types.ts
	@echo "$(GREEN)Type generation complete.$(RESET)"

# ── Testing ────────────────────────────────────────────────────────────────────
.PHONY: test
test: test-backend ## Run all tests

.PHONY: test-backend
test-backend: ## Run Go tests
	cd $(BACKEND) && go test ./... -v

# ── Linting ────────────────────────────────────────────────────────────────────
.PHONY: lint
lint: lint-backend lint-frontend ## Run all linters

.PHONY: lint-backend
lint-backend: ## Run golangci-lint
	cd $(BACKEND) && golangci-lint run ./...

.PHONY: lint-frontend
lint-frontend: ## Run TypeScript/React lint
	cd $(FRONTEND) && npx tsc --noEmit

# ── Docker ─────────────────────────────────────────────────────────────────────
.PHONY: docker-up
docker-up: ## Start production stack
	docker compose -f docker/docker-compose.yml up -d --build

.PHONY: docker-up-dev
docker-up-dev: ## Start dev stack with hot-reload volumes
	docker compose -f docker/docker-compose.dev.yml up --build

.PHONY: docker-down
docker-down: ## Stop all containers (prod + dev + test)
	docker compose -f docker/docker-compose.yml down
	docker compose -f docker/docker-compose.dev.yml down 2>/dev/null || true
	docker compose -f docker/docker-compose.test.yml down 2>/dev/null || true

.PHONY: docker-build-test
docker-build-test: ## Build the test image (pre-build for faster pre-commit hooks)
	docker compose -f docker/docker-compose.test.yml build

.PHONY: docker-test-backend
docker-test-backend: ## Run Go lint + tests inside the test container
	docker compose -f docker/docker-compose.test.yml run --rm backend-check

.PHONY: docker-test-frontend
docker-test-frontend: ## Run TypeScript check inside the test container
	docker compose -f docker/docker-compose.test.yml run --rm frontend-check

# ── Setup ──────────────────────────────────────────────────────────────────────
.PHONY: setup
setup: ## Install all dev tools and dependencies
	@echo "$(YELLOW)Installing Go tools (system)...$(RESET)"
	go install github.com/air-verse/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "$(YELLOW)Installing frontend dependencies...$(RESET)"
	cd $(FRONTEND) && npm install
	@echo "$(YELLOW)Setting up git hooks...$(RESET)"
	git config core.hooksPath .githooks
	@echo "$(YELLOW)Pre-building Docker test image...$(RESET)"
	$(MAKE) docker-build-test
	@echo "$(GREEN)Setup complete. You only need Docker to develop.$(RESET)"

# ── Cleanup ────────────────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove all build artifacts
	rm -rf $(BACKEND)/bin $(BACKEND)/tmp $(BACKEND)/static $(BACKEND)/docs
	rm -rf $(FRONTEND)/dist
