<p align="center">
  <a href="https://github.com/anonto42/home_network_DNS_management_system">
    <img src="./docs/logo.svg" width="120" height="120" alt="OmniDNS Logo">
  </a>
</p>

<h1 align="center">OmniDNS</h1>

<p align="center">
  A self-hosted DNS server for your home network with a full web dashboard. It filters ads and trackers, lets you create local DNS records, logs every query from every device on your network, and gives you traffic steering rules — all manageable from a browser.
</p>

<p align="center">
  <a href="#-quick-start">🌐 Quick Start</a> •
  <a href="#-features">📖 Features</a> •
  <a href="./BRANDING.md">🎨 Branding Guide</a> •
  <a href="#-self-hosting">🚀 Self-Hosting</a>
</p>

<p align="center">
  ⚡ Fast Local Resolution · 🛡️ Domain Blocklist · 🎛️ Traffic Steering · 📊 Real-time Monitoring
</p>

---

## 🌐 Quick Start

OmniDNS runs completely inside Docker for development. The stack includes a Go backend with **Air hot-reload** and a React+Vite frontend with **HMR**.

### 1. Clone and Set Up
```bash
git clone https://github.com/anonto42/home_network_DNS_management_system.git
cd home_network_DNS_management_system

# One-time setup: registers git hooks and pre-builds check images
make setup
```

### 2. Run the Stack
```bash
make dev
```
*   **Web Dashboard:** [http://localhost:5173](http://localhost:5173) (Proxies `/api` to the backend)
*   **API Server:** `http://localhost:8080`
*   **DNS Resolver:** Port `5354` (UDP) on host (mapped to `5353` internally to prevent host conflicts)

### 3. Development Credentials
The Docker development stack seeds a local admin account:
*   **Email:** `admin@omnidns.local`
*   **Password:** `change-me-in-dev`

For production, set `OMNIDNS_ADMIN_PASSWORD` before first start. The server refuses to create an initial admin account without it.

---

## 📖 Features

### 🛡️ Blocklist Management
Protect your network from tracking, advertising, and telemetry.
*   Block domains, subdomains, and wildcard expressions (`*.telemetry.microsoft.com`).
*   Configurable response behaviors (returns `0.0.0.0` or `NXDOMAIN`).
*   Instantly exports list to CSV.

### 🎛️ Traffic Steering Rules
Route specific traffic according to custom rules:
*   **Match Types:** Client IP, Domain prefix/suffix/regex, Protocol, or Query Type.
*   **Action Types:** Block, Rewrite, Forward to specific upstream, or Mock answer.

### 📝 Local DNS Records
Define authoritative local DNS records for your home laboratory and local devices:
*   Supports `A`, `AAAA`, `CNAME`, `TXT`, and `MX` records.
*   Detects duplicate conflicts and enables quick search.

---

## 🚀 Self-Hosting

Builds a single minimal Alpine image (~20 MB) with the Go binary and the compiled React frontend embedded inside it.

```bash
# Build the production image
make build

# Set a private initial admin password, then start the production stack
export OMNIDNS_ADMIN_PASSWORD='replace-with-a-long-private-password'
make up
# → Dashboard at http://localhost:8080
# → DNS on port 53 (host networking)

# Stop
make down
```

---

<details>
<summary><b>🛠️ Full Technical Documentation (Click to expand)</b></summary>

### How It Works

```
Device asks: "What is the IP for google.com?"
         │
         ▼
   Your Router (192.168.1.1)
         │  forwards DNS queries to OmniDNS
         ▼
   OmniDNS DNS (192.168.1.X:53)
         │
         ├─ Is the domain on the blocklist?
         │     YES → return 0.0.0.0 (blocked)
         │
         ├─ Is there a custom local record?
         │     YES → return your local IP (e.g. 192.168.1.50)
         │
         ├─ Is it in the in-memory cache?
         │     YES → return cached answer instantly
         │
         └─ Forward to upstream (Cloudflare 1.1.1.1 / Google 8.8.8.8)
               → cache the answer
               → return it to the device
```

### Useful Development Commands
```bash
make logs        # tail live logs from both containers
make dev-down    # stop the dev stack
make test        # run Go lint + unit tests + TypeScript typecheck
make test-backend   # Go checks only
make test-frontend  # TypeScript typecheck only
```

### Non-Docker Build Instructions
Only needed if you cannot use Docker.
```bash
# 1. Build the frontend
cd frontend
npm install
npm run build        # outputs to frontend/dist/
cd ..

# 2. Build the Go binary with the frontend embedded
cd backend
go build -tags embed -o ../omnidns-dns ./cmd/dns-server
cd ..

# 3. Run (port 53 requires root or cap_net_bind_service)
sudo ./omnidns-dns
```

</details>

<details>
<summary><b>🔌 API & CLI Reference (Click to expand)</b></summary>

### CLI Flags
All configurations can be controlled via CLI flags:
*   `--dns-port` (default `53`): DNS listening port.
*   `--http-port` (default `8080`): API and web dashboard listening port.
*   `--upstream` (default `1.1.1.1:53`): Fallback upstream DNS server.
*   `--log-level` (default `info`): Log levels: `debug`, `info`, `warn`, `error`.
*   `--log-prune` (default `168h`): How long to keep query logs (e.g., `24h`, `168h`).
*   `--admin-email`: Initial admin email. Can also be set with `OMNIDNS_ADMIN_EMAIL`.
*   `--admin-password`: Initial admin password. Can also be set with `OMNIDNS_ADMIN_PASSWORD`.

### API Endpoints
All dashboard interactions use JSON API endpoints. Authentication uses bearer session tokens returned by `POST /api/login`.

*   `POST /api/login` - Authenticate user
*   `POST /api/logout` - Clear session
*   `GET /api/session` - Get current session
*   `GET /api/stats` - Fetch real-time DNS stats
*   `GET /api/logs` - Query logs (paginated)
*   `GET /api/records` - List custom DNS records
*   `POST /api/records` - Create local DNS record
*   `DELETE /api/records` - Remove local DNS record

</details>
