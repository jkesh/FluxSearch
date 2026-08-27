#!/usr/bin/env bash
# 停止 deploy-local.sh 启动的进程
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../" && pwd)"
cd "$ROOT"

# shellcheck source=deploy/scripts/lib/load-env.sh
source "${ROOT}/deploy/scripts/lib/load-env.sh"
fluxsearch_load_config "$ROOT"

PID_DIR="${FLUXSEARCH_DEPLOY_PID_DIR:-.deploy/pids}"

stop_one() {
  local name="$1"
  local pidfile="${PID_DIR}/${name}.pid"
  if [[ ! -f "$pidfile" ]]; then
    return 0
  fi
  local pid
  pid="$(cat "$pidfile")"
  if kill -0 "$pid" 2>/dev/null; then
    echo "[stop] ${name} PID ${pid}"
    kill "$pid" 2>/dev/null || true
    sleep 1
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$pidfile"
}

for svc in frontend api worker monitor flagembedding; do
  stop_one "$svc"
done

echo "完成。"
