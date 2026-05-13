# Todo List

Monorepo: Go backend + React frontend. Production serves both via single binary on :8080.

## Stack

- **Backend**: Go + Echo v4 + GORM + SQLite/MySQL/PostgreSQL
- **Frontend**: Vite + React + TypeScript + Ant Design + TanStack Query
- **Auth**: basic (form login) or OIDC, configurable via `config.yaml`
- **Build**: Makefile targets, multi-stage Dockerfile

## Development

```bash
# Backend (from repo root)
cd backend && go run cmd/server/main.go -config ../config.yaml

# Frontend (from repo root)
cd frontend && npm run dev

# Production build
make build
```

## Key Commands

- `make build` — full production build (frontend + backend binary with embedded SPA)
- `make docker-build` — build Docker image `graydovee/todolist`
- `make backend-dev` — run backend with hot reload
- `make frontend-dev` — run Vite dev server on :5173

## Architecture

- `backend/internal/config/` — YAML config with env var override
- `backend/internal/database/` — GORM + SQL migrations per dialect
- `backend/internal/model/` — GORM models (User, Todo, TodoTag, TodoRelation, CodeCounter, Session)
- `backend/internal/repository/` — Data access layer
- `backend/internal/service/` — Domain logic (code generation, cycle detection, cascade)
- `backend/internal/handler/` — Echo HTTP handlers + DTOs
- `backend/internal/middleware/` — Auth, CSRF, CORS, logging
- `backend/internal/session/` — DB-backed session store
- `backend/internal/auth/` — Basic + OIDC auth providers
- `backend/static/` — Embedded frontend dist
- `frontend/src/` — React app with Ant Design

## Domain Rules

- Categories: bug/feature/task (immutable after creation)
- Codes: BUG-N / FEATURE-N / TASK-N per-user auto-increment
- Tags: trim + lowercase, dedup per todo
- Dependencies: depends_on (DAG), duplicate_of (single canonical target)
- Complete: cascade depends_on + auto-complete duplicates
- Reopen: cascade dependents + auto-reopen duplicates
- Delete: hard delete, orphan children, clean relations
