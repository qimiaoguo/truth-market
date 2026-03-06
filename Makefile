.PHONY: all lint test test-all test-infra test-services test-pact test-bench \
        coverage proto openapi-lint openapi-bundle docs-dev docker-up docker-down \
        docker-test-up docker-test-down migrate ci clean build

# Variables
MODULES = pkg infra services/gateway services/auth-svc services/market-svc services/trading-svc services/ranking-svc
PROTO_DIR = proto
GEN_DIR = proto/gen

# ──────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────

all: lint test build

build:
	@echo "==> Building all modules..."
	@for mod in $(MODULES); do \
		echo "  Building $$mod..."; \
		(cd $$mod && go build ./...) || exit 1; \
	done

clean:
	@echo "==> Cleaning build artifacts..."
	@for mod in $(MODULES); do \
		(cd $$mod && go clean ./...); \
	done
	rm -rf $(GEN_DIR)

# ──────────────────────────────────────────────────
# Lint
# ──────────────────────────────────────────────────

lint: lint-go lint-proto lint-openapi

lint-go:
	@echo "==> Running golangci-lint..."
	@for mod in $(MODULES); do \
		echo "  Linting $$mod..."; \
		(cd $$mod && golangci-lint run ./...) || exit 1; \
	done

lint-proto:
	@echo "==> Running buf lint..."
	@(cd $(PROTO_DIR) && buf lint)

lint-openapi:
	@echo "==> Running OpenAPI lint..."
	@npx @redocly/cli lint api/openapi.yaml

# ──────────────────────────────────────────────────
# Test
# ──────────────────────────────────────────────────

test:
	@echo "==> Running unit tests (short mode)..."
	@for mod in $(MODULES); do \
		echo "  Testing $$mod..."; \
		(cd $$mod && go test ./... -v -race -short -count=1) || exit 1; \
	done

test-all:
	@echo "==> Running all tests (including integration)..."
	@for mod in $(MODULES); do \
		echo "  Testing $$mod..."; \
		(cd $$mod && go test ./... -v -race -count=1) || exit 1; \
	done

test-infra:
	@echo "==> Running infra tests (requires Docker)..."
	@(cd infra && go test ./... -v -race -count=1)

test-services:
	@echo "==> Running service tests..."
	@for svc in gateway auth-svc market-svc trading-svc ranking-svc; do \
		echo "  Testing $$svc..."; \
		(cd services/$$svc && go test ./... -v -race -short -count=1) || exit 1; \
	done

test-pact:
	@echo "==> Running Pact contract tests..."
	@for svc in gateway auth-svc market-svc trading-svc ranking-svc; do \
		echo "  Pact testing $$svc..."; \
		(cd services/$$svc && go test ./... -v -tags=pact -count=1) || exit 1; \
	done

test-bench:
	@echo "==> Running benchmark tests..."
	@(cd services/trading-svc && go test ./internal/matching/... -bench=. -benchmem -run=^$$)

# ──────────────────────────────────────────────────
# Coverage
# ──────────────────────────────────────────────────

coverage:
	@echo "==> Generating coverage report..."
	@mkdir -p .coverage
	@for mod in $(MODULES); do \
		modname=$$(echo $$mod | tr '/' '-'); \
		echo "  Coverage for $$mod..."; \
		(cd $$mod && go test ./... -short -coverprofile=../.coverage/$$modname.out) || true; \
	done
	@echo "==> Coverage summary:"
	@for f in .coverage/*.out; do \
		echo "--- $$f ---"; \
		go tool cover -func=$$f | grep total || true; \
	done

# ──────────────────────────────────────────────────
# Proto
# ──────────────────────────────────────────────────

proto:
	@echo "==> Generating protobuf code..."
	@(cd $(PROTO_DIR) && buf generate)
	@echo "==> Done."

proto-breaking:
	@echo "==> Checking protobuf breaking changes..."
	@(cd $(PROTO_DIR) && buf breaking --against '.git#branch=main')

# ──────────────────────────────────────────────────
# OpenAPI
# ──────────────────────────────────────────────────

openapi-lint:
	@echo "==> Linting OpenAPI spec..."
	@npx @redocly/cli lint api/openapi.yaml

openapi-bundle:
	@echo "==> Bundling OpenAPI spec..."
	@npx @redocly/cli bundle api/openapi.yaml --output api/bundled.yaml

# ──────────────────────────────────────────────────
# Docs
# ──────────────────────────────────────────────────

docs-dev:
	@echo "==> Starting Mintlify dev server..."
	@(cd docs && pnpm mintlify dev)

# ──────────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────────

docker-up:
	@echo "==> Starting development environment..."
	docker compose up -d

docker-down:
	@echo "==> Stopping development environment..."
	docker compose down

docker-test-up:
	@echo "==> Starting test environment..."
	docker compose -f docker-compose.test.yml up -d

docker-test-down:
	@echo "==> Stopping test environment..."
	docker compose -f docker-compose.test.yml down

# ──────────────────────────────────────────────────
# Migrate
# ──────────────────────────────────────────────────

MIGRATE_URL ?= postgres://truthmarket:truthmarket@localhost:5432/truthmarket?sslmode=disable

migrate-up:
	@echo "==> Running migrations..."
	@migrate -database "$(MIGRATE_URL)" -path migrations up

migrate-down:
	@echo "==> Rolling back last migration..."
	@migrate -database "$(MIGRATE_URL)" -path migrations down 1

migrate-create:
	@echo "==> Creating new migration: $(NAME)"
	@migrate create -ext sql -dir migrations -seq $(NAME)

# ──────────────────────────────────────────────────
# CI (simulate full pipeline)
# ──────────────────────────────────────────────────

ci: lint test
	@echo "==> CI pipeline passed!"
