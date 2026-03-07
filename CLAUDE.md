# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Truth Market is a prediction market platform for humans and AI agents. Go + Next.js monorepo with five backend microservices communicating via gRPC, a gateway exposing REST + WebSocket, and a Next.js 15 frontend.

## Common Commands

### Build & Test (from repo root)
```bash
make build              # Build all Go modules
make test               # Unit tests (short mode, race detector)
make test-all           # All tests including integration
make test-infra         # Infra tests (requires Docker/testcontainers)
make test-bench         # Matching engine benchmarks
make lint               # Full lint (Go + proto + OpenAPI)
make ci                 # Simulate full CI pipeline (lint + test)
```

### Single module test
```bash
cd services/trading-svc && go test ./... -v -race -short -count=1
cd pkg && go test ./... -v -race -short -count=1
```

### Protobuf
```bash
make proto              # Generate Go gRPC stubs (uses buf v2)
make proto-breaking     # Check breaking changes against main
```

### Migrations (golang-migrate)
```bash
make migrate-up
make migrate-down
make migrate-create NAME=add_something
```

### Frontend (from `frontend/`, uses pnpm 9.15.0)
```bash
pnpm dev                # Dev server on port 3000
pnpm build              # Production build
pnpm lint               # ESLint
pnpm test:run           # Vitest single run
pnpm test:e2e           # Playwright
```

### Docker
```bash
make docker-up          # Full dev stack (postgres, redis, otel, jaeger, prometheus)
make docker-test-up     # Test infra only (postgres:5433, redis:6380)
```

## Architecture

### Go Workspace

`go.work` links all modules. Each service has its own `go.mod` with `replace` directives for local `pkg`, `infra`, and `proto/gen/go` modules. Docker builds use `GOWORK=off`.

### Service Layout

| Service | Port | Role |
|---|---|---|
| gateway | 8080 (HTTP) | REST API + WebSocket, translates to gRPC calls |
| auth-svc | 9001 (gRPC) | JWT auth via SIWE (Sign-In With Ethereum), API keys |
| market-svc | 9002 (gRPC) | Market lifecycle (draft/open/closed/resolved/cancelled) |
| trading-svc | 9003 (gRPC) | Order matching engine, trade execution |
| ranking-svc | 9004 (gRPC) | Leaderboard and portfolio aggregation |

The gateway is the single external entry point. No service mesh; plain gRPC between services.

### Internal Package Convention

Each service follows: `cmd/` (entrypoint), `internal/config/`, `internal/service/`, `internal/grpc/`.

### Repository Pattern

- `pkg/repository/` defines interfaces (`UserRepository`, `MarketRepository`, `OrderRepository`, etc.)
- `pkg/domain/` has shared domain types (`User`, `Market`, `Order`, `Trade`, `Position`, `Ranking`)
- `infra/postgres/` — production implementations using pgx/v5
- `infra/redis/` — session store, rate limiter, event bus, orderbook cache, ranking cache
- `infra/memory/` — in-memory implementations for unit tests (no Docker needed)

### Event Bus

Redis Pub/Sub via `pkg/eventbus`. Topics: `trade.executed`, `order.placed`, `order.cancelled`, `market.created`, `market.resolved`, `balance.updated`. Gateway subscribes and forwards to WebSocket clients on `market:<id>` and `user:<id>` channels.

### Matching Engine

In-process per-outcome orderbook in `services/trading-svc/internal/matching/`. Price-time priority (FIFO), self-trade prevention, prices constrained to `[0.01, 0.99]`.

### Authentication

Two methods (validated by gateway calling auth-svc):
1. **JWT** — SIWE flow: `GET /auth/nonce` -> `POST /auth/verify` -> HS256 JWT (24h), `Authorization: Bearer <token>`
2. **API Key** — `X-API-Key` header, takes precedence when both present

New users get 1000 unit initial balance. User types: `human` (wallet) and `agent` (bot/AI).

### API Conventions

- Envelope format: `{ ok, data?, error?: { code, message }, meta?: { page, per_page, total } }`
- All monetary values serialized as decimal strings (e.g., `"0.65"`)

## Key Tech Stack

**Backend:** Go 1.25, Gin, gRPC, pgx/v5 (PostgreSQL 17), go-redis/v9 (Redis 7), shopspring/decimal, OpenTelemetry, golangci-lint

**Frontend:** Next.js 15 (App Router), React 19, Tailwind CSS v4, Zustand 5, TanStack React Query v5, wagmi v3 + viem v2 (wallet), TypeScript strict mode

**Infra:** PostgreSQL 17, Redis 7, buf v2 (protobuf), golang-migrate, Docker Compose

## Frontend Structure

- `src/app/` — App Router pages (home, market/[id], portfolio, rankings, admin)
- `src/components/` — UI components organized by domain (market/, orderbook/, trading/, ranking/, wallet/)
- `src/hooks/` — React Query hooks for API data fetching
- `src/lib/api.ts` — `ApiClient` singleton wrapping fetch with JWT auth, talks to `NEXT_PUBLIC_API_URL` (default `http://localhost:8080/api/v1`)
- `src/stores/authStore.ts` — Zustand auth store persisted to localStorage

## Database

Migrations in `migrations/` using sequential numbering (`000001_create_users.up.sql`). Default connection: `postgres://truthmarket:truthmarket@localhost:5432/truthmarket?sslmode=disable`.

## Environment Variables

Gateway: `PORT` (8080), `AUTH_SVC_ADDR`, `MARKET_SVC_ADDR`, `TRADING_SVC_ADDR`, `RANKING_SVC_ADDR`, `REDIS_ADDR`, `JWT_SECRET`, `OTEL_ENDPOINT`. Each backend service: `PORT`, `DATABASE_URL`, `REDIS_ADDR`, `OTEL_ENDPOINT`. Frontend: `NEXT_PUBLIC_API_URL`.

## Workflows

**Every fix or feature must follow these rules. No exceptions.**

1. **TDD is mandatory** — write test first, see it fail (RED), then implement (GREEN), then refactor. Never write implementation before the test.
2. **All automated tests must pass** — run the relevant test suite, lint, and build. Fix any failures before proceeding.
3. **End-to-end verification required** — after tests pass, start the actual services (`docker compose up -d --build`) and verify the feature works for real (curl API / open browser). Evidence required.

### Workflow Documents

Read the relevant document before starting work:

- **`DEV_WORKFLOW.md`** — full development workflow: TDD phases, test scope decision table, test utilities reference, end-to-end verification steps, completion checklist. **Read this before starting any fix or feature.**
- **`CODE_REVIEW.md`** — self-review checklist (architecture, error handling, concurrency, naming) and PR review checklist (scope, coverage, dependencies, performance). **Read this after completing implementation.**
- **`SECURITY_REVIEW.md`** — comprehensive security audit: injection prevention, auth/authz, input validation, sensitive data, dependency security, Docker/gRPC/WebSocket security, rate limiting. **Read this when changes touch auth, data handling, APIs, or infrastructure.**
