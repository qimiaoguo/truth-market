#!/usr/bin/env bash
# Kill running dev Go services by PID file and port-based fallback.
# Used by: make dev-down, make dev-destroy, and scripts/dev.sh (startup cleanup).
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PID_FILE="$ROOT_DIR/.dev/pids"
PORTS=(9001 9002 9003 9004 8080)

killed=false

# 1) Kill by PID file (primary — kills go run parent processes)
if [[ -f "$PID_FILE" ]]; then
    while read -r pid; do
        [[ -z "$pid" ]] && continue
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            killed=true
        fi
    done < "$PID_FILE"
    rm -f "$PID_FILE"
fi

# 2) Kill by port (fallback — catches orphaned child processes from go run)
for port in "${PORTS[@]}"; do
    pids=$(lsof -ti :"$port" 2>/dev/null || true)
    if [[ -n "$pids" ]]; then
        echo "    Killing process on port $port (PID: $(echo $pids | tr '\n' ' '))"
        echo "$pids" | xargs kill 2>/dev/null || true
        killed=true
    fi
done

# Brief wait for processes to release ports
if [[ "$killed" == true ]]; then
    sleep 1
fi
