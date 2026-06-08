# 🌐 DNS Server & Dashboard

A high-performance, ad-blocking DNS server and management dashboard built with Go and React. Designed for speed, reliability, and ease of use.

## 🚀 Features

- **DNS Engine:** Standards-compliant implementation using `miekg/dns`.
- **Ad Blocking:** Block trackers and ad domains with wildcard support.
- **Custom Records:** Easily map hostnames to local IPs (e.g., `nas.local` → `192.168.1.50`).
- **In-Memory Speed:** Blocklists and custom records are cached in memory for near-zero latency.
- **Batch Logging:** High-throughput logging via buffered channel flushes to SQLite.
- **Modern Dashboard:** Real-time log table, stats cards, and easy record management.
- **Single Binary:** Embedded frontend assets for simple deployment.

## 🏗️ Architecture

- **Backend:** Go 1.22, SQLite (WAL mode), `chi` router, `miekg/dns`.
- **Frontend:** React 18, Vite, Tailwind CSS, TypeScript.
- **Orchestration:** Monorepo structure managed by `go.work` and a central `Makefile`.

## 📂 Project Structure

```text
.
├── backend/          # Go DNS & API Service
├── frontend/         # React Dashboard Application
├── .agents/          # AI Agent workspace for documentation & history
├── go.work           # Go workspace configuration
├── Makefile          # Central command hub
└── BLUEPRINT.md      # Detailed architectural guide
```

## 🛠️ Getting Started

### 1. Setup Environment
Ensure you have Go 1.22+ and Node.js 20+ installed.

```bash
make setup
```

### 2. Development Mode
Starts the Go backend (with Air hot-reload) and Vite frontend (with HMR) in parallel.

```bash
make dev
```

### 3. Generate Types
Automatically sync Go models with frontend TypeScript interfaces.

```bash
make generate
```

### 4. Build for Production
Creates a single, self-contained binary with the frontend embedded.

```bash
make build-prod
```

## 🐳 Docker

Run the entire stack in containers:

```bash
# Development (with hot-reload)
make docker-up-dev

# Production
make docker-up
```

## 📝 Configuration
Copy `.env.example` to `.env` and adjust settings as needed.

- `DNS_PORT`: Port to listen for DNS queries (default: 53)
- `HTTP_PORT`: Port for the API and Dashboard (default: 8080)
- `UPSTREAM_DNS`: Upstream server for non-blocked queries (default: 1.1.1.1:53)

---
*Built with ❤️ for home networks.*
