# Development Workflow

Every fix or feature MUST follow this workflow. No exceptions.

## Iron Laws

1. **TDD is mandatory** — write test first, see RED, then implement
2. **All tests must pass** — no skipping, no "will fix later"
3. **End-to-end verification required** — start services, verify the feature works for real

## Phase 1: TDD (RED → GREEN → REFACTOR)

### Step 1: Write a Failing Test (RED)

Before writing any implementation code:

1. Create a test file (or add to existing) for the change
2. Write test(s) that describe the expected behavior
3. Run the test — **it MUST fail**. If it passes, your test proves nothing.
4. Commit the failing test (optional but recommended)

### Step 2: Write Minimal Implementation (GREEN)

1. Write the smallest amount of code that makes the test pass
2. Run the test — **it MUST pass now**
3. Do not add unrequested features

### Step 3: Refactor

1. Clean up code while keeping tests green
2. Run tests after each refactor to confirm nothing broke

### Red Flags — STOP and start over if:

- You wrote implementation code before the test
- Your test passed on first run (test doesn't test anything)
- You're thinking "this is too simple to test" (test it anyway)
- You wrote the test after the implementation "just to satisfy the rule"

## Phase 2: Run Automated Tests

### Test Scope Decision Table

| What Changed | Commands to Run |
|---|---|
| `pkg/` or `infra/` (shared code) | `make test` (all modules) |
| Single service only | `cd services/<svc> && go test ./... -v -race -short -count=1` |
| Frontend only | `cd frontend && pnpm test:run && pnpm build` |
| Proto definitions | `make proto && make test` |
| Multiple areas | `make test` (all modules) |
| Any change | `make lint` (always run lint) |

### Test Utilities (use these, don't reinvent)

**Go Backend:**
- `infra/testutil/fixtures.go` — factory functions: `NewUser()`, `NewAgent()`, `NewMarket()`, `NewBinaryMarket()`, `NewOrder()`, `NewTrade()` with options like `WithBalance()`, `WithStatus()`
- `infra/testutil/containers.go` — `PostgresContainer()`, `RedisContainer()` for integration tests with testcontainers
- `infra/testutil/assertions.go` — `AssertDecimalEqual()`, `AssertBalanceEqual()`, `AssertErrorCode()`
- `infra/memory/` — in-memory implementations of all repository interfaces (no Docker needed for unit tests)
- Use `testify/assert` and `testify/require` for assertions

**Frontend:**
- Vitest + `@testing-library/react` + `@testing-library/user-event`
- Setup file: `src/test/setup.ts`
- MSW (Mock Service Worker) for API mocking
- Playwright for e2e tests (`pnpm test:e2e`)

## Phase 3: End-to-End Verification

After all automated tests pass, verify the feature works for real. Choose the appropriate method:

### Option A: Integration Tests (preferred for backend)

Run service-level integration tests that use testcontainers (real postgres + redis, no image build needed):

```bash
# Trading/order changes — full settlement flow with real DB
make test-trading-integration

# All infra integration tests
make test-infra
```

These tests verify real SQL execution, transactions, and constraints — not mocks.

### Option B: Local Dev Services (for manual debugging / curl / browser)

Start all Go services locally with `go run` (no Docker image build):

```bash
make dev          # Starts postgres+redis containers + all 5 services
# Gateway available at http://localhost:8080
# Press Ctrl+C to stop services

make dev-down     # Stop everything (containers + go processes)
```

Then verify:

```bash
# Confirm gateway is healthy
curl -s localhost:8080/api/v1/markets | head -20

# Test specific endpoints
curl -s localhost:8080/api/v1/markets/{id}
curl -s -X POST localhost:8080/api/v1/orders -H "Authorization: Bearer <token>" -d '...'
```

For frontend changes, also start the dev server:

```bash
cd frontend && pnpm dev   # http://localhost:3000
```

### Option C: Full Docker Compose (pre-release only)

Build all service images — only needed for final smoke test before deployment:

```bash
docker compose up -d --build
```

### Evidence

You MUST have concrete evidence before claiming completion:
- Integration test output showing PASS
- curl response showing correct data
- Screenshot of the UI working

**"It should work" is not evidence. Run it and show the output.**

## Completion Checklist

Before claiming a fix/feature is done:

- [ ] Wrote tests (unit or integration) for the change
- [ ] Tests went RED → GREEN (watched them fail first)
- [ ] Module-specific tests pass (`go test` or `pnpm test:run`)
- [ ] Lint passes (`make lint` or `pnpm lint`)
- [ ] Build passes (`pnpm build` if frontend changed)
- [ ] End-to-end verification done (integration test / `make dev` + curl / browser)
- [ ] Have evidence (command output or screenshot)
