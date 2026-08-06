#!/usr/bin/env bash
# gitsss-run.sh
# Kill any stale gitsss processes bound to the port (so old PIDs never hold
# port 3000), then start the server. Usage:
#   ./scripts/gitsss-run.sh                 # default port 3000, exe gitsss, args "web"
#   PORT=3001 EXE=gitsss ./scripts/gitsss-run.sh web --port 3001
set -euo pipefail

PORT="${PORT:-3000}"
EXE="${EXE:-gitsss}"
EXTRA_ARGS=("$@")
if [ ${#EXTRA_ARGS[@]} -eq 0 ]; then
  EXTRA_ARGS=("web")
fi

echo "[gitsss-run] cleaning stale processes on port $PORT ..."

# Kill processes that own the listening port (Linux / macOS)
if command -v fuser >/dev/null 2>&1; then
  fuser -k "${PORT}/tcp" 2>/dev/null || true
elif command -v lsof >/dev/null 2>&1; then
  pids=$(lsof -ti tcp:"$PORT" 2>/dev/null || true)
  [ -n "$pids" ] && kill -9 $pids 2>/dev/null || true
fi

# Kill any leftover gitsss* processes (covers stale builds from earlier runs)
pkill -9 -f "${EXE}" 2>/dev/null || true

sleep 1

echo "[gitsss-run] starting ${EXE} ${EXTRA_ARGS[*]}"
exec "./${EXE}" "${EXTRA_ARGS[@]}"
