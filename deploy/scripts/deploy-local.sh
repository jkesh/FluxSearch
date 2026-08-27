#!/usr/bin/env bash
# FluxSearch 本地一键启动（FlagEmbedding + API + Monitor + 前端）
# 用法: bash deploy/scripts/deploy-local.sh
# 停止: bash deploy/scripts/stop-local.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../" && pwd)"
cd "$ROOT"

# shellcheck source=deploy/scripts/lib/load-env.sh
source "${ROOT}/deploy/scripts/lib/load-env.sh"

fluxsearch_init_config "$ROOT"
fluxsearch_load_config "$ROOT"

LOG_DIR="${FLUXSEARCH_DEPLOY_LOG_DIR:-.deploy/logs}"
PID_DIR="${FLUXSEARCH_DEPLOY_PID_DIR:-.deploy/pids}"
mkdir -p "$LOG_DIR" "$PID_DIR"

is_running() {
  local name="$1"
  local pidfile="${PID_DIR}/${name}.pid"
  if [[ -f "$pidfile" ]]; then
    local pid
    pid="$(cat "$pidfile")"
    if kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    rm -f "$pidfile"
  fi
  return 1
}

start_bg() {
  local name="$1"
  shift
  if is_running "$name"; then
    echo "[skip] ${name} 已在运行 (PID $(cat "${PID_DIR}/${name}.pid"))"
    return 0
  fi
  echo "[start] ${name}"
  nohup "$@" >"${LOG_DIR}/${name}.log" 2>&1 &
  echo $! >"${PID_DIR}/${name}.pid"
  sleep 1
  if ! kill -0 "$(cat "${PID_DIR}/${name}.pid")" 2>/dev/null; then
    echo "[fail] ${name} 启动失败，见 ${LOG_DIR}/${name}.log"
    tail -n 20 "${LOG_DIR}/${name}.log" 2>/dev/null || true
    return 1
  fi
  echo "[ok] ${name} PID $(cat "${PID_DIR}/${name}.pid")"
}

if [[ "${FLUXSEARCH_DEPLOY_BUILD:-false}" == "true" ]]; then
  echo "=== 构建 ==="
  make build
  if [[ "${FLUXSEARCH_DEPLOY_START_FRONTEND:-true}" == "true" ]]; then
    make build-frontend
  fi
fi

if [[ "${FLUXSEARCH_DEPLOY_START_FLAGEMBEDDING:-true}" == "true" ]] \
  && [[ "${FLUXSEARCH_FLAGEMBEDDING_ENABLED:-true}" == "true" ]]; then
  start_bg flagembedding python "${ROOT}/scripts/flagembedding_server.py"
fi

if [[ "${FLUXSEARCH_DEPLOY_START_MONITOR:-true}" == "true" ]]; then
  start_bg monitor go run ./cmd/monitor
fi

if [[ "${FLUXSEARCH_DEPLOY_START_API:-true}" == "true" ]]; then
  start_bg api go run ./cmd/api
fi

if [[ "${FLUXSEARCH_DEPLOY_START_WORKER:-false}" == "true" ]]; then
  start_bg worker go run ./cmd/worker
fi

if [[ "${FLUXSEARCH_DEPLOY_START_FRONTEND:-true}" == "true" ]]; then
  start_bg frontend bash -lc "cd frontend && npm run dev"
fi

API_PORT="${FLUXSEARCH_API_PORT:-8080}"
FE_PORT="${FLUXSEARCH_FRONTEND_PORT:-5173}"
MON_PORT="${FLUXSEARCH_MONITOR_PORT:-8090}"
FE_URL="${FLUXSEARCH_FLAGEMBEDDING_HOST:-127.0.0.1}"
FLAG_PORT="${FLUXSEARCH_FLAGEMBEDDING_PORT:-8091}"

echo ""
echo "=== FluxSearch 已启动 ==="
echo "  前端:     http://127.0.0.1:${FE_PORT}"
echo "  API:      http://127.0.0.1:${API_PORT}"
echo "  Monitor:  http://127.0.0.1:${MON_PORT}/api/v1/status"
echo "  FlagEmb:  http://${FE_URL}:${FLAG_PORT}/v1"
echo "  日志目录: ${LOG_DIR}"
echo "  停止服务: bash deploy/scripts/stop-local.sh"
echo ""
