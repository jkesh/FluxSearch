#!/bin/bash
# 在服务器上运行（需 sudo）
# 用法: sudo bash server-port-forward.sh
set -euo pipefail

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
NS=fluxsearch

if ! command -v kubectl &>/dev/null; then
  echo "错误: 未找到 kubectl"
  exit 1
fi

if ! kubectl get ns "$NS" &>/dev/null; then
  echo "错误: 无法访问命名空间 $NS，请检查 KUBECONFIG"
  exit 1
fi

echo "=== 停止旧转发 ==="
pkill -f "kubectl port-forward -n ${NS}" 2>/dev/null || true
sleep 2

start_forward() {
  local svc=$1
  local port=$2
  local log="/tmp/pf-${svc}.log"

  if ss -tln | grep -q ":${port} "; then
    echo "警告: 端口 ${port} 已被占用，跳过 ${svc}"
    ss -tlnp | grep ":${port} " || true
    return 1
  fi

  # 尝试绑定 0.0.0.0（允许局域网访问）
  if kubectl port-forward -n "$NS" --address 0.0.0.0 "svc/${svc}" "${port}:${port}" \
      >"$log" 2>&1 &
  then
    echo "启动 ${svc} -> 0.0.0.0:${port} (PID $!)"
    return 0
  fi

  echo "失败 ${svc}，日志:"
  cat "$log" 2>/dev/null || true
  return 1
}

echo ""
echo "=== 启动转发 ==="
start_forward postgres 5432 || true
start_forward redis    6379 || true
start_forward minio    9000 || true
start_forward milvus   19530 || true
start_forward etcd     2379 || true

sleep 3

echo ""
echo "=== 检查监听端口 ==="
for port in 5432 6379 9000 19530 2379; do
  if ss -tln | grep -q ":${port} "; then
    echo "  OK  :${port}"
  else
    echo "  FAIL:${port}"
  fi
done

echo ""
echo "=== 进程 ==="
pgrep -af "port-forward -n ${NS}" || echo "无 port-forward 进程"

echo ""
echo "=== 错误日志（如有）==="
for f in /tmp/pf-*.log; do
  [ -f "$f" ] || continue
  if grep -qiE 'error|fail|unable' "$f" 2>/dev/null; then
    echo "--- $f ---"
    cat "$f"
  fi
done

echo ""
echo "完成。从局域网访问: http://$(hostname -I | awk '{print $1}'):<port>"
