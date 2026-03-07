# Security Review

Comprehensive security audit checklist for the Truth Market platform. Run through relevant sections when making changes that touch authentication, data handling, APIs, or infrastructure.

## Injection Prevention

- [ ] **SQL injection** — all queries use pgx parameterized queries (`$1, $2`), never string formatting
- [ ] **XSS** — React auto-escapes by default; no `dangerouslySetInnerHTML` without sanitization
- [ ] **Command injection** — no `exec.Command` with user-supplied input
- [ ] **gRPC input** — proto field sizes bounded; no unbounded repeated fields from external input
- [ ] **Log injection** — user input not interpolated directly into log format strings

## Authentication & Authorization

- [ ] **JWT validation** — tokens verified with correct secret, expiry checked, algorithm enforced (HS256)
- [ ] **SIWE verification** — nonce validated and consumed (single-use, 5-min TTL in Redis)
- [ ] **API Key validation** — keys hashed in database, compared with constant-time comparison
- [ ] **Auth middleware** — all protected routes go through gateway auth middleware before reaching handlers
- [ ] **Admin checks** — admin-only endpoints verify `user.is_admin`, not just authenticated
- [ ] **Resource ownership** — users can only access/modify their own orders, positions, API keys
- [ ] **API Key precedence** — when both JWT and API Key present, API Key takes precedence (documented behavior)

## Input Validation

- [ ] **Price range** — enforced `[0.01, 0.99]` at both API handler and matching engine level
- [ ] **Quantity** — positive integers, reasonable upper bound
- [ ] **UUID format** — market IDs, outcome IDs, user IDs validated as UUIDs before database queries
- [ ] **Market status transitions** — only valid transitions allowed (`draft→open→closed→resolved|cancelled`)
- [ ] **Decimal precision** — `shopspring/decimal` used, no float64 for monetary values
- [ ] **Pagination** — `page` and `per_page` have reasonable defaults and upper bounds

## Sensitive Data Protection

- [ ] **JWT secret** — loaded from environment variable, not hardcoded
- [ ] **No secrets in logs** — tokens, API keys, passwords never logged
- [ ] **No secrets in errors** — error responses don't leak internal details to clients
- [ ] **No secrets in git** — `.env` files in `.gitignore`, no credentials committed
- [ ] **Wallet addresses** — treated as non-sensitive public data (by design in Ethereum)
- [ ] **Balance information** — only accessible to the owning user

## Dependency Security

```bash
# Go: check for known vulnerabilities
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Frontend: audit npm packages
cd frontend && pnpm audit

# Check outdated dependencies
go list -m -u all
cd frontend && pnpm outdated
```

- [ ] No dependencies with known critical CVEs
- [ ] Dependencies pinned to specific versions (not `latest`)
- [ ] `go.sum` and `pnpm-lock.yaml` committed and up-to-date

## Docker & Infrastructure Security

- [ ] **Non-root user** — services run as `appuser` in containers (check Dockerfile `USER` directive)
- [ ] **Minimal base image** — Alpine-based images, no unnecessary tools
- [ ] **No secrets in images** — no `.env` files, credentials, or keys baked into Docker images
- [ ] **Multi-stage builds** — build tools not present in production image
- [ ] **Network isolation** — services communicate on internal Docker network only
- [ ] **Database credentials** — not using default `truthmarket/truthmarket` in production

## gRPC Security

- [ ] **Internal only** — gRPC ports (9001-9004) not exposed externally, only gateway is public
- [ ] **Input size limits** — gRPC max message size configured to prevent OOM
- [ ] **No reflection in production** — gRPC reflection disabled in production builds
- [ ] **Error details** — gRPC errors don't leak stack traces or internal paths to clients

## Rate Limiting

- [ ] **Auth endpoints** — 10 req/min per IP (prevent brute force)
- [ ] **Trading endpoints** — 60 req/min per user (prevent abuse)
- [ ] **Market/Rankings** — 120 req/min per IP or user
- [ ] **Rate limiter bypass** — no way to skip rate limiting via header manipulation
- [ ] **Redis failure** — rate limiting degrades gracefully (allows requests, logs warning)

## Event Bus Security

- [ ] **No sensitive data in events** — Redis Pub/Sub events don't contain tokens or secrets
- [ ] **Event validation** — consumers validate event payload before processing
- [ ] **No replay attacks** — events are idempotent or have deduplication

## WebSocket Security

- [ ] **Authentication required** — WebSocket connections require valid JWT
- [ ] **Channel isolation** — users can only subscribe to `user:<own_id>`, not others
- [ ] **Message size limits** — WebSocket messages bounded to prevent memory exhaustion
- [ ] **Connection limits** — max connections per user to prevent resource exhaustion

## CORS & Browser Security

- [ ] **CORS origin** — restricted to known frontend origins, not `*` in production
- [ ] **Content-Type** — API responses set correct `Content-Type: application/json`
- [ ] **HTTPS** — enforced in production (TLS termination at load balancer)
