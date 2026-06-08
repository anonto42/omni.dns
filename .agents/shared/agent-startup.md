# Agent Startup — DNS Monorepo

> Read this file first. Then read `caveman.md`. Then start working.

## Project

DNS server with a web dashboard. Go backend + React frontend. Single binary in
production (frontend embedded via `//go:embed`). No firmware — that was removed.

## Stack

| Layer     | Tech                                      |
|-----------|-------------------------------------------|
| Backend   | Go 1.22, chi, miekg/dns (target), SQLite  |
| Frontend  | React 18, Vite 5, TypeScript, Tailwind    |
| Dev tools | Air (hot reload), Swag (OpenAPI), Vitest  |
| Container | Docker multi-stage, docker-compose        |

## Repo layout (flattened)

```
.
├── backend/          ← Go service
├── frontend/apps/web-app/
├── .agents/shared/   ← YOU ARE HERE
├── Makefile          ← central command hub
├── go.work           ← use ./backend
└── .env.example
```

## Key commands

```bash
make setup          # install all tools + deps
make dev            # Air + Vite in parallel
make generate       # Go → OpenAPI → api-types.ts
make build-prod     # React build → embed into Go binary
make test           # Go tests + Vitest
make lint           # golangci-lint + eslint
make docker-up-dev  # hot-reload Docker stack
```

## Rules (non-negotiable)

1. Never use anonymous structs in API handlers — always named structs in `models/types.go`
2. Always run `make generate` after touching `internal/` Go files — commit the result
3. `api-types.ts` is generated — never hand-edit it
4. All DNS hot-path code must be non-blocking — no SQLite calls in `dns/handler.go`
5. `docs/` is gitignored — generated on every `make generate`
6. After every `git commit`, immediately run `make lint` and `make test`. If checks fail, resolve all issues before proceeding to `git push`.
7. **Commit Messages:** All commits MUST contain a clear, concise summary of "why" the change was made, providing sufficient context.
8. **AI Sign-off:** Every AI-generated commit MUST be signed off using the current Git user: `Signed-off-by: Md Sohidul Islam Ananto <anontom90@gmail.com>`.

## Next: read `caveman.md` for current state

## Next: read `caveman.md` for current state
