#!/usr/bin/env bash
# 为 K8s 服务配置公网/节点 IP 访问（externalIPs + NodePort Monitor）
# 在服务器上运行: sudo bash deploy/scripts/apply-external-access.sh
# 或: FLUXSEARCH_PUBLIC_IP=113.128.132.69 bash deploy/scripts/apply-external-access.sh
set -euo pipefail

export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
NS="${FLUXSEARCH_K8S_NAMESPACE:-fluxsearch}"
PUBLIC_IP="${FLUXSEARCH_PUBLIC_IP:-113.128.132.69}"

if ! command -v kubectl &>/dev/null; then
  echo "错误: 未找到 kubectl"
  exit 1
fi

if ! kubectl get ns "$NS" &>/dev/null; then
  echo "错误: 命名空间 $NS 不存在"
  exit 1
fi

echo "=== FluxSearch 外网访问配置 ==="
echo "PUBLIC_IP=${PUBLIC_IP}"
echo "NAMESPACE=${NS}"
echo ""

echo "=== 停止 kubectl port-forward（避免与节点端口冲突）==="
pkill -f "kubectl port-forward -n ${NS}" 2>/dev/null || true
sleep 2

patch_external_ip() {
  local svc="$1"
  if ! kubectl get svc "$svc" -n "$NS" &>/dev/null; then
    echo "[skip] svc/${svc} 不存在"
    return 0
  fi
  kubectl patch svc "$svc" -n "$NS" --type merge \
    -p "{\"spec\":{\"externalIPs\":[\"${PUBLIC_IP}\"]}}"
  echo "[ok] svc/${svc} externalIPs=${PUBLIC_IP}"
}

echo "=== 为 ClusterIP 服务添加 externalIPs ==="
for svc in postgres redis minio milvus etcd; do
  patch_external_ip "$svc"
done

# Helm 部署时服务名可能不同
for svc in fluxsearch-pg-postgresql fluxsearch-redis-master; do
  patch_external_ip "$svc"
done

echo ""
echo "=== Monitor Service（NodePort 30090 + externalIPs）==="
if kubectl get svc fluxsearch-monitor -n "$NS" &>/dev/null; then
  kubectl patch svc fluxsearch-monitor -n "$NS" --type merge -p "$(cat <<EOF
{
  "spec": {
    "type": "NodePort",
    "externalIPs": ["${PUBLIC_IP}"],
    "ports": [
      {
        "name": "http",
        "port": 8090,
        "targetPort": 8090,
        "nodePort": 30090
      }
    ]
  }
}
EOF
)"
  echo "[ok] svc/fluxsearch-monitor NodePort 30090 + externalIPs"
else
  echo "[skip] svc/fluxsearch-monitor 不存在（可使用 server-run-monitor.sh 在宿主机 8090）"
fi

echo ""
echo "=== 当前 Service ==="
kubectl get svc -n "$NS" -o wide

echo ""
echo "=== 监听端口（宿主机）==="
ss -tlnp 2>/dev/null | egrep ':22 |:5432 |:6379 |:9000 |:19530 |:2379 |:8090 |:30090 ' || true

echo ""
echo "=== 防火墙（如启用 ufw）==="
if command -v ufw &>/dev/null && ufw status 2>/dev/null | grep -qi active; then
  for port in 5432 6379 9000 19530 2379 8090 30090; do
    ufw allow "${port}/tcp" 2>/dev/null || true
  done
  echo "已尝试 ufw allow 上述端口"
else
  echo "ufw 未启用；若在云主机请在安全组放行: 5432,6379,9000,19530,2379,8090,30090"
fi

echo ""
echo "=== 集群外连接地址（${PUBLIC_IP}）==="
echo "  PostgreSQL  ${PUBLIC_IP}:5432"
echo "  Redis       ${PUBLIC_IP}:6379"
echo "  MinIO       ${PUBLIC_IP}:9000"
echo "  Milvus      ${PUBLIC_IP}:19530"
echo "  etcd        ${PUBLIC_IP}:2379"
echo "  Monitor     http://${PUBLIC_IP}:8090/api/v1/status  （宿主机进程）"
echo "  Monitor K8s http://${PUBLIC_IP}:30090/api/v1/status （NodePort）"
echo ""
echo "本地 config/local/infra.env 请设置:"
echo "  FLUXSEARCH_SERVER_HOST=${PUBLIC_IP}"
echo "  FLUXSEARCH_*_HOST_LOCAL=${PUBLIC_IP}"
echo "  FLUXSEARCH_MONITOR_URL=http://${PUBLIC_IP}:8090/api/v1/status"
echo ""
echo "完成。"
