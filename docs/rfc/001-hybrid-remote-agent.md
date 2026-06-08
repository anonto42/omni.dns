# RFC 001: Hybrid Remote Agent Architecture

**Date:** 2026-06-08
**Status:** Draft

### Objective
Enable out-of-home protection and cellular filtering without requiring a full, battery-draining VPN.

### Proposal
Develop a lightweight "remote agent" (e.g., a CLI tool or mobile companion app) that maintains an encrypted tunnel to the local DNS server, allowing remote devices to point their DNS queries to the home network over a secure, optimized connection.

### Technical Challenges
*   Security of the remote connection.
*   Battery efficiency on mobile devices.
*   Handling dynamic IP addresses (DDNS).
