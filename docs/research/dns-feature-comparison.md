# Research: DNS Feature Comparison (Local vs. Cloud)
**Date:** 2026-06-08
**Status:** Finalized

### Objective
Compare local self-hosted DNS servers with premium cloud-based DNS services to identify feature gaps and opportunities for our project's roadmap.

### Market/Context Analysis
The DNS market is currently bifurcated:
- **Local (Self-Hosted):** Privacy-focused, free, requires hardware management.
- **Cloud (Premium/Paid):** Feature-rich, mobile/remote protection, zero-maintenance.

### Technical Exploration: Feature Comparison

| Key Feature Evaluated | Pi-hole | AdGuard Home | Technitium | Unbound | NextDNS | Control D |
|---|---|---|---|---|---|---|
| Ad & Tracker Blocking | 1.0 | 1.0 | 0.5 | 0.0 | 1.0 | 1.0 |
| Web GUI Dashboard | 1.0 | 1.0 | 1.0 | 0.0 | 1.0 | 1.0 |
| DoH/DoT Encryption | 0.5 | 1.0 | 1.0 | 0.5 | 1.0 | 1.0 |
| Parental Controls | 0.0 | 1.0 | 0.0 | 0.0 | 1.0 | 1.0 |
| LAN Routing (Zones) | 0.5 | 0.5 | 1.0 | 0.5 | 0.0 | 1.0 |
| DHCP Server | 1.0 | 1.0 | 1.0 | 0.0 | 0.0 | 0.0 |
| Remote/Mobile Protection | 0.0 | 0.0 | 0.0 | 0.0 | 1.0 | 1.0 |
| Geo-Proxying | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 1.0 |
| Maintenance-Free | 0.0 | 0.0 | 0.0 | 0.0 | 1.0 | 1.0 |
| **⭐ TOTAL SCORE** | **4.0** | **5.5** | **5.5** | **1.0** | **7.0** | **8.0** |

### Key Insights
- Cloud services leverage "anywhere" protection via native device integration.
- Self-hosted services provide superior LAN control and privacy.
- There is a market gap for a self-hosted tool that bridges the "local control" of Pi-hole/AdGuard with the "remote accessibility" offered by cloud services.

### Conclusion & Next Steps
This analysis will inform our feature prioritization. We should focus on narrowing the gap between local self-hosted stability and remote access capabilities.
