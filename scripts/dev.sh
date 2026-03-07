#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="docker-compose.dev.yml"
PID_FILE="$ROOT_DIR/.dev/pids"
SERVICE_PORTS=(9001 9002 9003 9004 8080)

# Database config matching docker-compose.dev.yml
DB_USER="truthmarket_dev"
DB_PASS="truthmarket_dev"
DB_NAME="truthmarket_dev"
DB_PORT="5433"
REDIS_PORT="6380"

export DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
export REDIS_ADDR="localhost:${REDIS_PORT}"
export JWT_SECRET="dev-secret-change-me"
# Detect OrbStack: containers use .orb.local domains instead of localhost ports.
COMPOSE_PROJECT=$(basename "$ROOT_DIR")
if docker context show 2>/dev/null | grep -q orbstack; then
    IS_ORBSTACK=true
    CONTAINER_HOST_PREFIX="${COMPOSE_PROJECT}.orb.local"
    # OrbStack containers are accessible via <service>.<project>.orb.local
    export OTEL_ENDPOINT="otel-collector-dev.${CONTAINER_HOST_PREFIX}:4317"
else
    IS_ORBSTACK=false
    export OTEL_ENDPOINT="localhost:4317"
fi

# ── Handle --reset flag ──
if [[ "${1:-}" == "--reset" ]]; then
    echo "==> Resetting dev environment (destroying volumes)..."
    docker compose -f "$COMPOSE_FILE" down -v 2>/dev/null || true
    echo "    Done. Starting fresh."
fi

# ── Kill leftover processes from previous run ──
bash "$ROOT_DIR/scripts/dev-kill.sh"

# Track child PIDs for cleanup
mkdir -p "$(dirname "$PID_FILE")"
PIDS=()

cleanup() {
    echo ""
    echo "==> Shutting down Go services..."
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    # Also kill by port to catch orphaned go run child processes
    for port in "${SERVICE_PORTS[@]}"; do
        local_pids=$(lsof -ti :"$port" 2>/dev/null || true)
        if [[ -n "$local_pids" ]]; then
            echo "$local_pids" | xargs kill 2>/dev/null || true
        fi
    done
    wait 2>/dev/null
    rm -f "$PID_FILE"
    echo "==> Go services stopped. Containers still running (data persisted)."
    echo "    Run 'make dev-down' to stop containers, 'make dev-destroy' to wipe data."
}
trap cleanup EXIT INT TERM

# ── Step 1: Start all infra containers ──
echo "==> Starting infra containers (postgres, redis, migrate, otel, jaeger, prometheus)..."
docker compose -f "$COMPOSE_FILE" up -d

echo "==> Waiting for postgres to be healthy..."
for i in $(seq 1 30); do
    if docker compose -f "$COMPOSE_FILE" exec -T postgres-dev pg_isready -U "$DB_USER" &>/dev/null; then
        echo "    Postgres ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "ERROR: Postgres failed to start within 30s"
        exit 1
    fi
    sleep 1
done

echo "==> Waiting for redis to be healthy..."
for i in $(seq 1 15); do
    if docker compose -f "$COMPOSE_FILE" exec -T redis-dev redis-cli ping &>/dev/null; then
        echo "    Redis ready."
        break
    fi
    if [ "$i" -eq 15 ]; then
        echo "ERROR: Redis failed to start within 15s"
        exit 1
    fi
    sleep 1
done

# ── Step 2: Wait for migrations (runs as container, idempotent) ──
echo "==> Waiting for migrations to complete..."
docker compose -f "$COMPOSE_FILE" logs -f migrate-dev 2>&1 &
MIGRATE_LOG_PID=$!

for i in $(seq 1 30); do
    STATUS=$(docker compose -f "$COMPOSE_FILE" ps migrate-dev --format '{{.State}}' 2>/dev/null || echo "unknown")
    if [[ "$STATUS" == "exited" ]]; then
        EXIT_CODE=$(docker compose -f "$COMPOSE_FILE" ps migrate-dev --format '{{.ExitCode}}' 2>/dev/null || echo "1")
        kill "$MIGRATE_LOG_PID" 2>/dev/null || true
        if [[ "$EXIT_CODE" == "0" ]]; then
            echo "    Migrations complete."
        else
            echo "WARNING: Migrations exited with code ${EXIT_CODE} (may already be applied)."
        fi
        break
    fi
    if [ "$i" -eq 30 ]; then
        kill "$MIGRATE_LOG_PID" 2>/dev/null || true
        echo "WARNING: Migration timeout — services may fail if tables don't exist."
    fi
    sleep 1
done

# ── Step 3: Start Go services ──
echo "==> Starting Go services..."

start_service() {
    local name=$1
    local dir=$2
    local port=$3

    echo "    Starting ${name} on port ${port}..."
    PORT="$port" DATABASE_URL="$DATABASE_URL" REDIS_ADDR="$REDIS_ADDR" \
        JWT_SECRET="$JWT_SECRET" OTEL_ENDPOINT="$OTEL_ENDPOINT" \
        go run "./${dir}/cmd/" &
    PIDS+=($!)
    echo "$!" >> "$PID_FILE"
}

start_service "auth-svc"    "services/auth-svc"    9001
start_service "market-svc"  "services/market-svc"  9002
start_service "trading-svc" "services/trading-svc"  9003
start_service "ranking-svc" "services/ranking-svc"  9004

# Give backend services a moment to start before gateway
sleep 2

# Gateway connects to backend services (uses localhost defaults)
echo "    Starting gateway on port 8080..."
PORT=8080 REDIS_ADDR="$REDIS_ADDR" JWT_SECRET="$JWT_SECRET" OTEL_ENDPOINT="$OTEL_ENDPOINT" \
    go run ./services/gateway/cmd/ &
PIDS+=($!)
echo "$!" >> "$PID_FILE"

# ── Step 4: Wait for gateway ──
echo "==> Waiting for gateway..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/api/v1/markets &>/dev/null; then
        echo ""
        echo "============================================"
        echo "  All services running!"
        echo ""
        echo "  Gateway:     http://localhost:8080"
        echo "  API:         http://localhost:8080/api/v1"
        if [ "$IS_ORBSTACK" = true ]; then
            echo "  Jaeger UI:   https://jaeger-dev.${CONTAINER_HOST_PREFIX}"
            echo "  Prometheus:  https://prometheus-dev.${CONTAINER_HOST_PREFIX}"
        else
            echo "  Jaeger UI:   http://localhost:16686"
            echo "  Prometheus:  http://localhost:9090"
        fi
        echo "  Postgres:    localhost:${DB_PORT}"
        echo "  Redis:       localhost:${REDIS_PORT}"
        echo ""
        echo "  Data is persisted between restarts."
        echo "  Use 'make dev-reset' to wipe and start fresh."
        echo "  Press Ctrl+C to stop Go services."
        echo "============================================"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "WARNING: Gateway not responding yet. Services may still be starting..."
    fi
    sleep 1
done

# Keep running until interrupted
wait
