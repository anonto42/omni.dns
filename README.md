# DNS Server

A full-featured DNS server for your home network, running on an ESP32 with MicroPython.

## Features

- **DNS Server** — Handles DNS queries on port 53
- **Query Forwarding** — Unresolved domains forwarded to Cloudflare (1.1.1.1)
- **Ad Blocking** — Blocks known tracker/ads domains (customizable)
- **Custom Records** — Map domains to local IPs (e.g. `mydevice.local → 192.168.1.100`)
- **Query Logging** — See every DNS request live in the dashboard
- **Web Dashboard** — Stats, logs, manage records & blocklist from browser

## Project Structure

| File | Purpose |
|------|---------|
| `main.py` | Entry point — connects WiFi, starts DNS + web server |
| `dns_server.py` | Core DNS protocol — parses queries, builds responses, forwards upstream |
| `dns_records.py` | Custom records & blocklist management (persisted to JSON files) |
| `logger.py` | Query log buffer (keeps last 200 entries in memory) |
| `web_server.py` | HTTP server — serves dashboard HTML + REST API |
| `static/index.html` | Dashboard UI (served as-is from ESP32 flash) |
| `blocklist.json` | Persisted blocked domains list |
| `custom_records.json` | Persisted custom DNS mappings |

## How DNS Works (Learning)

1. A device (phone/laptop) asks: "What is the IP of `google.com`?"
2. This ESP32 catches the query (UDP packet on port 53)
3. **Check blocklist** — if domain is blocked → return `0.0.0.0` (nowhere)
4. **Check custom records** — if you mapped it → return your IP
5. **Forward** — otherwise → send query to Cloudflare (1.1.1.1), return their answer
6. **Log** — every query is recorded with timestamp, client IP, domain, action

## Setup

### 1. Install MicroPython on ESP32

Connect ESP32 via USB, then:

```bash
# Install esptool
pip install esptool

# Erase flash
esptool.py --port /dev/ttyUSB0 erase_flash

# Flash MicroPython (download .bin from micropython.org)
esptool.py --port /dev/ttyUSB0 --baud 460800 write_flash -z 0x1000 esp32-*.bin
```

### 2. Install mpremote (to upload files)

```bash
pip install mpremote
```

### 3. Configure WiFi

Edit `main.py` and set your WiFi credentials:

```python
WIFI_SSID = "YourNetworkName"
WIFI_PASS = "YourPassword"
```

### 4. Upload files to ESP32

```bash
mpremote cp main.py :main.py
mpremote cp dns_server.py :dns_server.py
mpremote cp dns_records.py :dns_records.py
mpremote cp logger.py :logger.py
mpremote cp web_server.py :web_server.py
mpremote mkdir :static
mpremote cp static/index.html :static/index.html
```

Or upload everything at once:

```bash
mpremote cp -r . :
```

### 5. Reset ESP32

```bash
mpremote reset
```

## Configure Your Router

Set your router's DNS server to the ESP32's IP address:

1. Find ESP32's IP in serial output (or check router DHCP client list)
2. In your router admin panel, set **DHCP DNS Server** → ESP32 IP
3. All devices on your WiFi will now use this DNS server

## Dashboard

Open `http://<esp32-ip>` in your browser:

- **Live Logs** tab — see every DNS query in real-time
- **Custom Records** — add local hostnames (e.g. `nas.local → 192.168.1.50`)
- **Blocklist** — add/remove domains to block

## Testing on Linux (Before ESP32)

Run the DNS server locally to test:

```bash
# Requires Python 3
pip install -r requirements.txt
python test_local.py
```

## Notes

- ESP32 has limited RAM (~200KB free). The log buffer holds 200 entries.
- Blocklist + custom records are persisted in flash (JSON files).
- For 1000+ blocklist entries, load them from a file at boot.
- ESP32 must be powered 24/7 if it's your primary DNS server.
# home_network_DNS_management_system
