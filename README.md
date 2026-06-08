# NetShield DNS

**Enterprise-Grade Self-Hosted DNS Server & Dashboard.**

NetShield DNS is a modern, performance-oriented DNS server designed for privacy, local network routing, and advanced ad-blocking. It combines a high-performance Go backend with a professional, Stitch Design-inspired React dashboard to give you full visibility and control over your network traffic.

---

## 🚀 Key Features

*   **⚡ High-Performance DNS:** Powered by `miekg/dns` for reliable, low-latency resolution.
*   **🛡️ Ad & Tracker Blocking:** In-memory, thread-safe blocklist management for instant resolution dropping.
*   **🔐 Encrypted Upstream:** Outbound DNS-over-TLS (DoT) support for secure communication with resolvers.
*   **🏠 LAN Routing:** Authoritative local domain hosting and custom A/AAAA/CNAME record management.
*   **📊 Enterprise Dashboard:** Professional monitoring, real-time query logging, and system health metrics via a modern React interface.
*   **📦 Production-Ready:** Single-binary deployment with embedded frontend assets.
*   **🚀 Persistent Service:** Designed to auto-start on boot as a background service on Linux, Windows, or macOS.

---

## 🏗️ Architecture Overview

NetShield DNS follows a monorepo structure:
- **`backend/`**: Go application managing DNS resolution, API endpoints, SQLite persistence, and stats.
- **`frontend/`**: React application using TailwindCSS and the Stitch Design system for the dashboard UI.
- **`docs/`**: Centralized architectural decisions, RFCs, and research.

---

## 🛠️ Getting Started

### Prerequisites
*   Docker & Docker Compose
*   Make

### Development Workflow
Start the full development stack (Backend + Frontend):
```bash
make docker-up-dev
```
*The dashboard will be available at `http://localhost:5173`.*

### Production Build
To generate the production-ready single binary:
```bash
make build-prod
```

---

## ⚙️ Deployment as a System Service

To enable auto-start on Linux/Raspberry Pi:
1.  Copy `docs/assets/dns-server.service` to `/etc/systemd/system/`.
2.  Adjust `User`, `Group`, and `ExecStart` paths.
3.  Run:
    ```bash
    sudo systemctl enable dns-server
    sudo systemctl start dns-server
    ```

---

## 📄 License & Contributing

This project is licensed under the **MIT License**. See the `LICENSE` file for details.

We welcome contributions! Please check the `CONTRIBUTING.md` file for workflows, verification standards, and architectural guidelines before submitting pull requests.
