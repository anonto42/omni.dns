# Full Project Plan — ESP32 DNS Server

## Overview

Three parts, one system:

```
┌─────────────────────────────────────────────────────────┐
│                    React Dashboard                      │
│  (Vite + Tailwind + TypeScript)                        │
│  └─ http://localhost:5173                               │
└──────────────────────┬──────────────────────────────────┘
                       │ REST API (JSON)
                       ▼
┌─────────────────────────────────────────────────────────┐
│                   Go Backend (server/)                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐  │
│  │ DNS      │  │ REST API │  │ SQLite               │  │
│  │ Server   │  │ (Chi)    │  │ (logs, records,      │  │
│  │ :53      │  │ :8080    │  │  blocklist)          │  │
│  └──────────┘  └──────────┘  └──────────────────────┘  │
└──────────────────────┬──────────────────────────────────┘
                       │ UDP port 53
                       ▼
┌─────────────────────────────────────────────────────────┐
│              ESP32 Rust Firmware (firmware/)            │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐  │
│  │ DNS      │  │ WiFi STA │  │ OLED + LEDs + Buttons│  │
│  │ Server   │  │ + Relay  │  │ (I2C, GPIO)          │  │
│  └──────────┘  └──────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

---

## Part 1 — Go Backend (`server/`)

### What it does

- Listens on UDP port 53 for DNS queries
- Parses DNS packets manually (no huge framework, we learn the protocol)
- Checks blocklist → returns NXDOMAIN or 0.0.0.0
- Checks custom records → returns your IP
- Forwards unresolved queries to 1.1.1.1
- Caches responses (LRU with TTL)
- Logs everything to SQLite
- Serves REST API on port 8080

### Files to build

| File | What it does |
|------|-------------|
| `cmd/dns-server/main.go` | Entry point: flags, init DB, start DNS + HTTP server |
| `internal/models/types.go` | Shared structs: Query, Record, BlockEntry, Stats |
| `internal/dns/handler.go` | Parse raw DNS packet, dispatch to block/custom/forward |
| `internal/dns/cache.go` | LRU cache with TTL expiry |
| `internal/dns/forwarder.go` | UDP client to 1.1.1.1:53 |
| `internal/api/router.go` | Chi router setup |
| `internal/api/handlers.go` | GET/POST/DELETE for logs, records, blocklist, stats |
| `internal/db/sqlite.go` | Open, migrate, CRUD operations |
| `go.mod` | Dependencies |
| `Makefile` | Build, run, test |

### Step-by-step

1. Initialize Go module
2. Define models (QueryLog, CustomRecord, BlockedDomain)
3. Build DNS handler: parse binary packet → extract domain → respond
4. Build forwarder: send raw query to 1.1.1.1 → return response
5. Add LRU cache: store recent responses, respect TTL
6. Add SQLite: schema, migrations, CRUD
7. Build REST API: Chi router, all endpoints
8. Wire everything in main.go
9. Create systemd service file
10. Add Prometheus metrics

---

## Part 2 — React Dashboard (`web/`)

### What it does

- Real-time view of DNS queries
- Stats cards (queries forwarded/blocked/custom)
- Live log table (auto-refresh)
- Manage custom records (add/remove domain→IP)
- Manage blocklist (add/remove domains)
- Clear logs
- Dark mode by default

### Files to build

| File | What it does |
|------|-------------|
| `src/App.tsx` | Root component, tabs (Logs, Records, Blocklist) |
| `src/api/client.ts` | Fetch wrapper for REST API |
| `src/hooks/usePolling.ts` | Custom hook for auto-refresh |
| `src/components/StatsCards.tsx` | 6 stats cards (status, IP, forwarded, blocked, custom, mem) |
| `src/components/LogTable.tsx` | Live DNS query log table |
| `src/components/RecordManager.tsx` | Add/remove custom DNS records |
| `src/components/BlocklistManager.tsx` | Add/remove blocked domains |
| `src/components/Tabs.tsx` | Tab navigation component |
| `src/pages/Dashboard.tsx` | Main dashboard layout |

### Setup

```bash
cd web
npm create vite@latest . -- --template react-ts
npm install tailwindcss @tailwindcss/vite
```

---

## Part 3 — ESP32 Rust Firmware (`firmware/`)

### What it does

Runs entirely on the ESP32 — no Linux machine needed.

- WiFi station mode (connects to your router)
- DNS server on port 53 (same logic as Go version)
- Query forwarding to 1.1.1.1
- Blocklist + custom records stored in flash (SPIFFS or NVS)
- OLED SSD1306 display (I2C) — shows stats
- RGB LED — green = running, red = blocking active
- Physical button — toggle blocklist on/off

### Files to build

| File | What it does |
|------|-------------|
| `Cargo.toml` | Dependencies (esp-idf-hal, embedded-svc, etc.) |
| `src/main.rs` | Entry point: init WiFi, start DNS + HTTP server |
| `src/dns.rs` | DNS packet parser and response builder |
| `src/wifi.rs` | WiFi connection manager with reconnect |
| `src/display.rs` | OLED SSD1306 driver (I2C) |
| `src/led.rs` | RGB LED controller |
| `src/button.rs` | Button debounce handler |
| `src/storage.rs` | NVS/SPIFFS for persisting blocklist + records |

### Setup (when ready)

```bash
# Install esp-rs toolchain
cargo install espup
espup install

# Install espflash
cargo install espflash

# Build and flash
cd firmware
cargo build
espflash flash target/esp32/debug/firmware.bin
```

---

## Build Order (Recommended)

```
Week 1:  Go DNS handler (parse, respond, forward)
         Test with: dig @localhost google.com

Week 2:  SQLite + REST API
         Test with: curl localhost:8080/api/status

Week 3:  React dashboard
         Test with: browser at localhost:5173

Week 4:  ESP32 Rust firmware
         Test with: set router DNS to ESP32 IP
```

---

## How DNS Packet Works (Reference)

```
DNS Query (binary, UDP port 53):

  Bytes 0-1:   Transaction ID (random)
  Bytes 2-3:   Flags (QR=0 for query)
  Bytes 4-5:   Questions count (usually 1)
  Bytes 6-7:   Answers count (0 for query)
  Bytes 8-9:   Authority count
  Bytes 10-11: Additional count
  Bytes 12+:   Question section
                 - Labels (len+data, 0-terminated)
                 - QTYPE (2 bytes, 1=A, 28=AAAA)
                 - QCLASS (2 bytes, 1=IN)

DNS Response:

  Same Transaction ID
  Flags: QR=1, AA=0, RCODE=0 (success) or 3 (NXDOMAIN)
  Question section repeated
  Answer section:
    - Name pointer (2 bytes: 0xC0 0x0C)
    - Type (2 bytes)
    - Class (2 bytes)
    - TTL (4 bytes)
    - Data length (2 bytes)
    - IP address (4 bytes for A record)
```
