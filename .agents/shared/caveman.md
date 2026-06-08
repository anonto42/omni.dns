# caveman.md — compressed state

> Dense notes. Append at top when state changes. Never delete lines.

---

## 2026-06-08 — initial scaffold

DONE:
- full directory structure flattened: `backend/` and `frontend/apps/web-app/`
- Makefile: comprehensive version implemented
- go.work: `use ./backend`
- .agents/shared/ workspace initialized

NOT DONE YET (work in this order):
1. Named request structs in `models/types.go`
2. Update handlers to use named structs
3. Implement `/health` endpoint in `router.go`
4. Batch log writer in `db/sqlite.go`
5. In-memory blocklist + custom_records maps
6. miekg/dns migration (Phase 6)
7. Single-binary embed (Phase 4)

ACTIVE TASK: Implementing named request structs and updating API handlers.

DECISIONS:
- Flattened backend path to reduce friction.
- Using named structs for robust type generation.

BLOCKERS: none
