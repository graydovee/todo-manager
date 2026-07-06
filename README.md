# Todo Manager

[English](README.md) | [简体中文](README.zh-CN.md)

A self-hostable todo tracker for individuals and small teams — Go backend, React frontend, single deployable binary. Track bugs, features, and tasks with auto-numbered codes, tags, dependency graphs, comments, and AI-assisted summaries.

Production builds compile the React SPA into the Go binary, so the whole app ships as one executable serving HTTP on `:8080`.

---

## Highlights

- **Typed todos** — every item is a `bug`, `feature`, or `task`, auto-numbered per user as `BUG-N` / `FEATURE-N` / `TASK-N`.
- **Dependencies & duplicates** — model work as a DAG with `depends_on`, or mark items as `duplicate_of` a canonical target. Completing or reopening an item cascades through its graph.
- **Dependency graph** — interactive visualization (React Flow + ELK layout) of how your todos relate.
- **Comments** — attach notes and progress updates to any todo.
- **Pin & highlight** — surface the work that matters right now.
- **AI summaries** — ask an LLM to summarize a todo (with its comments and related work) and ask follow-up questions, streamed back over SSE.
- **Flexible auth** — ship with built-in form login, or plug in an OIDC provider (Google, GitHub, Keycloak, …) for SSO.
- **API access keys** — issue scoped tokens (`todos:create`, `summaries:stream`, …) for automation and the CLI, separate from browser sessions.
- **Multi-database** — SQLite for zero-config local runs, MySQL or PostgreSQL for multi-replica deployments.
- **One binary, one container** — no runtime dependencies in the image (distroless, CGO-free).

## Tech stack

| Layer    | Choices |
|----------|---------|
| Backend  | Go 1.25 · Echo v4 · GORM · SQLite / MySQL / PostgreSQL · coreos/go-oidc · openai-go |
| Frontend | Vite · React 19 · TypeScript · Ant Design v6 · TanStack Query · React Flow · i18next |
| Auth     | Form login (session cookie) or OIDC · CSRF-protected · scoped API access keys |
| Build    | Makefile · multi-stage Dockerfile (distroless) · Helm chart |

## Quick start (development)

You'll need Go, Node.js, and npm on your PATH.

```bash
# 1. Create your local config from the template (config.yaml is gitignored)
cp config.example.yaml config.yaml

# 2. Start the backend (hot via go run) — serves :8080 by default
make backend-dev

# 3. In another terminal, start the Vite dev server — :5173
make frontend-dev
```

Open <http://localhost:5173>. The frontend proxies API calls to the backend. Default basic-auth users from the example config are `admin / admin123` and `user1 / user123`.

> The dev workflow uses two processes so you get Vite HMR on the frontend while the Go server runs separately. In production the built SPA is served by the Go binary itself (no Vite, no Node).

## Production build

```bash
# Builds the frontend, embeds it, and produces the server binary + CLI
make build

# Run it
make run          # ./bin/todo-manager -config config.yaml
```

Outputs land in `bin/`:

- `bin/todo-manager` — the server (SPA embedded)
- `bin/todo-cli` — the command-line client

## Docker

```bash
# Build the image locally
make docker-build          # tags graydovee/todo-manager:<version> and :latest

# Run with the bundled default config
docker run --rm -p 8080:8080 graydovee/todo-manager:latest
```

The image ships `config.example.yaml` as its default `/config.yaml`. To use your own config, mount it:

```bash
docker run --rm -p 8080:8080 -v "$PWD/config.yaml:/config.yaml" graydovee/todo-manager:latest
```

For SQLite persistence, mount a volume at `/data` (or wherever your `db.dsn` points).

Multi-arch (amd64 + arm64) images are published via `make release`.

## Kubernetes (Helm)

A Helm chart lives in [`charts/todo-manager/`](charts/todo-manager) and supports three database modes:

| Mode | When to use |
|------|-------------|
| `sqlite` | Single-replica, simple deployments. Optional PVC for persistence. |
| `bundled` | Deploys a PostgreSQL subchart alongside the app. Good default for HA. |
| `external` | Point at an existing Postgres or MySQL instance. |

```bash
# Default: SQLite, basic auth, one replica
helm install todo-manager ./charts/todo-manager

# With bundled PostgreSQL and an ingress
helm install todo-manager ./charts/todo-manager \
  --set database.mode=bundled \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=todos.example.com
```

See [`charts/todo-manager/values.yaml`](charts/todo-manager/values.yaml) for the full set of knobs (image, service, ingress, TLS, probes, resources, DB credentials).

## Configuration

All runtime config lives in a YAML file (default `config.yaml`, override with `-config`). Environment variables prefixed `TODO_MANAGER_` override the corresponding YAML keys — useful for injecting secrets in containers without writing them to disk.

| YAML path | Env var |
|-----------|---------|
| `server.port` | `TODO_MANAGER_SERVER_PORT` |
| `db.driver` | `TODO_MANAGER_DB_DRIVER` |
| `db.dsn` | `TODO_MANAGER_DB_DSN` |
| `auth.mode` | `TODO_MANAGER_AUTH_MODE` |
| `session.secret` | `TODO_MANAGER_SESSION_SECRET` |
| `auth.oidc.client_secret` | `TODO_MANAGER_OIDC_CLIENT_SECRET` |
| `llm.model` / `llm.base_url` / `llm.api_key` / `llm.timeout` | `TODO_MANAGER_LLM_*` |
| `log.format` / `log.level` | `TODO_MANAGER_LOG_*` |

A minimal config:

```yaml
server:
  port: 8080

db:
  driver: sqlite                       # sqlite | mysql | postgres
  dsn: "todo-manager.db"

auth:
  mode: basic                          # basic | oidc
  basic:
    users:
      - username: admin
        password: admin123
        display_name: Admin

session:
  secret: "change-me-in-production"    # long, random string
  max_age: 86400

llm:                                   # optional — powers AI summaries
  model: "gpt-4o"
  base_url: "https://api.openai.com"
  api_key: "sk-your-api-key-here"
  timeout: 30

log:
  format: text                         # text | json
  level: info                          # debug | info | warn | error
```

See [`config.example.yaml`](config.example.yaml) for the full schema including the OIDC block. **Change every default secret before deploying** — session secret, basic-auth passwords, and the LLM API key.

## Project structure

```
.
├── backend/                Go service (Echo + GORM)
│   ├── cmd/server/         Entry point — loads config, runs migrations, serves HTTP
│   └── internal/
│       ├── app/            Route wiring, SPA fallback
│       ├── auth/           Basic + OIDC providers
│       ├── authz           Permissions & access-key scopes
│       ├── config/         YAML config + TODO_MANAGER_* env overrides
│       ├── database/       GORM + per-dialect migrations
│       ├── handler/        HTTP handlers + DTOs
│       ├── middleware/     Auth, CSRF, CORS, permission checks
│       ├── model/          GORM models
│       ├── repository/     Data access layer
│       ├── service/        Domain logic (codes, DAG, cascade, summaries)
│       └── session/        DB-backed session store
├── frontend/               Vite + React + TypeScript + Ant Design
├── todo-cli/               Command-line client (talks to the REST API)
├── skills/                 Claude Code skill that drives `todo-cli`
├── charts/todo-manager/    Helm chart
├── config.example.yaml     Config template
├── Dockerfile              Multi-stage, distroless
└── Makefile                build / dev / test / docker / release targets
```

## Command-line client (`todo-cli`)

`todo-cli` is a Go client for the REST API — handy on its own and the backbone of the bundled Claude Code skill. After `make build` (or `make cli-build`), sign in once and start managing todos from the shell:

```bash
./bin/todo-cli login                    # authenticate against your server
./bin/todo-cli todo list --status open
./bin/todo-cli todo create --type bug --title "Rotate auth keys quarterly"
./bin/todo-cli todo complete TASK-7
```

Run `todo-cli --help` for the full command tree.

## REST API

All endpoints are prefixed `/api/v1`. Browser sessions and API access keys are both accepted on the resource routes (the `AuthEither` middleware), with each call checked against a fine-grained permission scope.

| Area        | Endpoints |
|-------------|-----------|
| Auth        | `GET /auth/mode`, `GET /auth/csrf`, `POST /auth/login`, `GET /auth/login` (OIDC redirect), `GET /auth/callback`, `POST /auth/logout`, `GET /auth/me` |
| Access keys | `GET/POST /access-keys`, `GET /access-keys/permissions`, `POST /access-keys/:id/rotate`, `DELETE /access-keys/:id` |
| Todos       | `GET /todos`, `GET /todos/graph`, `GET /todos/tags`, `GET /todos/by-date-range`, `GET/POST/PATCH/DELETE /todos[/:id]`, `POST /todos/:id/start\|complete\|reopen`, `PATCH /todos/:id/status\|pin\|highlight` |
| Comments    | `GET/POST /todos/:id/comments`, `DELETE /todos/:id/comments/:cid` |
| Summaries   | `POST /summaries`, `GET /summaries`, `GET /summaries/:id`, `GET /summaries/:id/stream` (SSE), `DELETE /summaries/:id` |
| Follow-ups  | `POST /summaries/:id/followup`, `GET /summaries/:id/followups` |
| Health      | `GET /health` |

`GET /todos` accepts filter/sort query params (status, category, tag, dependencies, paging). The `GET /todos/graph` endpoint returns nodes and edges for the dependency visualization.

## Testing

```bash
make test        # backend (go test) + frontend (vitest) + cli (go test)
make cli-test    # todo-cli only
```

Backend tests include property tests (via `pgregory.net/rapid`) and integration tests for the summary handler.

## Domain rules (quick reference)

- **Categories** are immutable after creation: `bug`, `feature`, `task`.
- **Codes** are per-user auto-increment: `BUG-N`, `FEATURE-N`, `TASK-N`.
- **Tags** are trimmed, lowercased, and de-duplicated within a todo.
- **Dependencies** form a DAG (`depends_on`); cycle creation is rejected.
- **Duplicates** point at a single canonical target (`duplicate_of`).
- **Complete** cascades through `depends_on` and auto-completes duplicates; **reopen** does the reverse.
- **Delete** is a hard delete: children are orphaned and relations cleaned up.

## Security notes

- `config.yaml` is gitignored on purpose — it's where live secrets live. Keep it that way. The committed [`config.example.yaml`](config.example.yaml) holds only placeholder values.
- Rotate the session secret, basic-auth passwords, and any API keys before exposing the app beyond your local machine.
- API access keys are scoped — grant the minimum permission set, and prefer them over reusing a browser session for automation.
