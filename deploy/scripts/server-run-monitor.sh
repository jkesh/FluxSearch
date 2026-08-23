#!/bin/bash
# 在服务器上后台运行 fluxsearch-monitor（集群内可直接访问中间件）
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/fluxsearch}"
BINARY="$INSTALL_DIR/fluxsearch-monitor"
LOG="/tmp/fluxsearch-monitor.log"

export FLUXSEARCH_POSTGRES_HOST="${FLUXSEARCH_POSTGRES_HOST:-postgres.fluxsearch.svc.cluster.local}"
export FLUXSEARCH_POSTGRES_PORT="${FLUXSEARCH_POSTGRES_PORT:-5432}"
export FLUXSEARCH_POSTGRES_USER="${FLUXSEARCH_POSTGRES_USER:-fluxsearch}"
export FLUXSEARCH_POSTGRES_PASSWORD="${FLUXSEARCH_POSTGRES_PASSWORD:-changeme-in-production}"
export FLUXSEARCH_POSTGRES_DB="${FLUXSEARCH_POSTGRES_DB:-fluxsearch}"

export FLUXSEARCH_REDIS_HOST="${FLUXSEARCH_REDIS_HOST:-redis.fluxsearch.svc.cluster.local}"
export FLUXSEARCH_REDIS_PORT="${FLUXSEARCH_REDIS_PORT:-6379}"
export FLUXSEARCH_REDIS_PASSWORD="${FLUXSEARCH_REDIS_PASSWORD:-changeme-in-production}"

export FLUXSEARCH_MINIO_ENDPOINT="${FLUXSEARCH_MINIO_ENDPOINT:-minio.fluxsearch.svc.cluster.local:9000}"
export FLUXSEARCH_MINIO_ACCESS_KEY="${FLUXSEARCH_MINIO_ACCESS_KEY:-fluxsearch}"
export FLUXSEARCH_MINIO_SECRET_KEY="${FLUXSEARCH_MINIO_SECRET_KEY:-changeme-in-production}"
export FLUXSEARCH_MINIO_BUCKET="${FLUXSEARCH_MINIO_BUCKET:-milvus-bucket}"

export FLUXSEARCH_MILVUS_HOST="${FLUXSEARCH_MILVUS_HOST:-milvus.fluxsearch.svc.cluster.local}"
export FLUXSEARCH_MILVUS_PORT="${FLUXSEARCH_MILVUS_PORT:-19530}"
export FLUXSEARCH_ETCD_HOST="${FLUXSEARCH_ETCD_HOST:-etcd.fluxsearch.svc.cluster.local}"
export FLUXSEARCH_ETCD_PORT="${FLUXSEARCH_ETCD_PORT:-2379}"

export FLUXSEARCH_MONITOR_ADDR="${FLUXSEARCH_MONITOR_ADDR:-0.0.0.0:8090}"

# 优先加载 ClusterIP 配置（主机无法解析 *.svc.cluster.local 时）
if [ -f "$INSTALL_DIR/monitor.env" ]; then
  set -a
  # shellcheck disable=SC1091
  source "$INSTALL_DIR/monitor.env"
  set +a
fi

if [ ! -x "$BINARY" ]; then
  echo "错误: 未找到 $BINARY，请先运行 deploy-monitor-binary.ps1"
  exit 1
fi

pkill -f fluxsearch-monitor 2>/dev/null || true
sleep 1
nohup "$BINARY" >"$LOG" 2>&1 &
sleep 2

if pgrep -f fluxsearch-monitor >/dev/null; then
  IP=$(hostname -I | awk '{print $1}')
  echo "Monitor 已启动: http://${IP}:8090/api/v1/status"
  echo "日志: $LOG"
  curl -s "http://127.0.0.1:8090/healthz" || true
else
  echo "启动失败，日志:"
  tail -20 "$LOG"
  exit 1
fi
