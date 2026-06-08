.PHONY: help build run test clean web web-dev fmt vet \
        docker-build docker-run docker-compose-up docker-compose-down \
        firmware-build firmware-flash all

# ─── Help ───────────────────────────────────────────────────────────────
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-24s\033[0m %s\n", $$1, $$2}'

# ─── Go Server ──────────────────────────────────────────────────────────
build: ## Build Go binary → bin/dns-server
	cd server && go build -o ../bin/dns-server ./cmd/dns-server

run: ## Run Go server (sudo for port 53)
	cd server && sudo go run ./cmd/dns-server

run-dev: ## Run Go server with dev flags (no root needed on high ports)
	cd server && go run ./cmd/dns-server --dns-port 5353 --http-port 8080

test: ## Run Go tests
	cd server && go test ./... -v

fmt: ## Format Go code
	cd server && go fmt ./...

vet: ## Run Go vet
	cd server && go vet ./...

clean: ## Remove build artifacts
	rm -rf bin/ data/

# ─── React Frontend ─────────────────────────────────────────────────────
web: ## Build React for production → web/dist
	cd web && npm run build

web-dev: ## Start Vite dev server (proxies /api to :8080)
	cd web && npm run dev

web-install: ## Install/update npm dependencies
	cd web && npm install

# ─── Docker ──────────────────────────────────────────────────────────────
docker-build: ## Build Docker image
	docker build -t dns-server:latest .

docker-run: ## Run container (host networking, needs port 53)
	docker run --rm --name dns-server \
		--cap-add=NET_BIND_SERVICE \
		--network host \
		-v dns-data:/app/data \
		dns-server:latest

docker-compose-up: ## Start via Docker Compose
	docker compose up -d --build

docker-compose-down: ## Stop Docker Compose
	docker compose down

docker-compose-logs: ## Tail container logs
	docker compose logs -f

docker-run-alt: ## Run on alternate ports (no root)
	docker run --rm --name dns-server \
		-p 5353:5353/udp -p 5353:5353 -p 8080:8080 \
		-v dns-data:/app/data \
		dns-server:latest \
		--dns-port 5353 --http-port 8080 --static web/dist

# ─── ESP32 Firmware ─────────────────────────────────────────────────────
firmware-build: ## Build Rust firmware
	cd firmware && cargo build

firmware-flash: ## Flash firmware to ESP32
	cd firmware && espflash flash

# ─── All-in-one ─────────────────────────────────────────────────────────
all: build web ## Build Go binary + React frontend

start: build run ## Build and run server

dev: web-dev ## Start frontend dev server (run server separately)

# ─── Docker Compose override ────────────────────────────────────────────
# Use:  make docker-compose-up DOCKER_PORT=5353
DOCKER_PORT ?= 53
docker-compose-up-alt:
	DOCKER_PORT=$(DOCKER_PORT) docker compose -f docker-compose.yml up -d --build
