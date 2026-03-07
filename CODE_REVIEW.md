# Code Review

Two scenarios: self-review after completing your own code, and reviewing others' PRs.

## Self-Review Checklist

Run through this checklist after finishing implementation, before claiming done.

### Architecture & Patterns

- [ ] Follows service layout: `cmd/`, `internal/config/`, `internal/service/`, `internal/grpc/`
- [ ] New repository methods added to interface in `pkg/repository/` first, then implemented
- [ ] Domain types in `pkg/domain/`, not defined locally in services
- [ ] Uses dependency injection — services receive repository interfaces, not concrete types
- [ ] Gateway handlers translate HTTP ↔ gRPC correctly

### Error Handling

- [ ] Go services return proper gRPC status codes (`codes.NotFound`, `codes.InvalidArgument`, etc.)
- [ ] Gateway translates gRPC errors to HTTP status codes with envelope format: `{ ok: false, error: { code, message } }`
- [ ] No swallowed errors — every error is either returned, logged, or handled
- [ ] Frontend displays user-friendly error messages from API responses

### Data & Business Logic

- [ ] Monetary values use `shopspring/decimal`, never float64
- [ ] API serializes monetary values as decimal strings (`"0.65"`, not `0.65`)
- [ ] Prices validated in range `[0.01, 0.99]`
- [ ] Database queries use parameterized queries (pgx), never string concatenation
- [ ] Transactions used where multiple writes must be atomic

### Concurrency & Performance

- [ ] No data races — shared state protected (mutex, channels, or single-goroutine ownership)
- [ ] No N+1 queries — batch load where possible
- [ ] No goroutine leaks — goroutines have shutdown signals
- [ ] Context propagation correct (using `ctx` from request, not `context.Background()`)

### Naming & Consistency

- [ ] Go naming follows conventions (unexported unless needed, no stuttering like `user.UserService`)
- [ ] Frontend components organized by domain (`market/`, `trading/`, `orderbook/`, etc.)
- [ ] React hooks in `hooks/` directory, API calls through `lib/api.ts`
- [ ] Zustand store updates use the established pattern in `stores/`

### Tests

- [ ] New code has corresponding tests
- [ ] Tests cover both happy path and error cases
- [ ] Tests use existing test utilities (`infra/testutil/`, `infra/memory/`)
- [ ] No test pollution — tests don't depend on execution order

## PR Review Checklist

When reviewing a pull request (yours or someone else's).

### Scope & Intent

- [ ] Changes match the PR title and description
- [ ] No unrelated changes bundled in
- [ ] Commit messages are clear and meaningful

### Coverage & Quality

- [ ] Tests added or updated for the change
- [ ] Test coverage adequate for new code paths
- [ ] Edge cases considered (empty inputs, boundary values, concurrent access)
- [ ] No TODO/FIXME left without a tracking issue

### Dependencies & Compatibility

- [ ] No unnecessary new dependencies added
- [ ] `go.mod` / `package.json` changes are intentional
- [ ] Proto changes checked with `make proto-breaking`
- [ ] API changes are backward-compatible (or documented as breaking)

### Performance

- [ ] Database queries are efficient (indexed columns, no full table scans)
- [ ] No unnecessary allocations in hot paths (matching engine)
- [ ] Redis operations batched where possible (pipeline)
- [ ] Frontend re-renders minimized (proper `key` props, memoization where needed)

### Observability

- [ ] Structured logging with `pkg/logger` (not `fmt.Println`)
- [ ] OpenTelemetry spans for cross-service calls
- [ ] Error logs include enough context to debug
