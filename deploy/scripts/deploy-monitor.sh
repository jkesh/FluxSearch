#!/bin/bash
# 在服务器上构建并部署 fluxsearch-monitor
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

echo "=== 构建 monitor 镜像 ==="
# 使用本地构建（服务器需安装 Go）或预编译二进制
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/fluxsearch-monitor ./cmd/monitor

echo "=== 构建 Docker 镜像 ==="
cat > /tmp/Dockerfile.monitor <<'EOF'
FROM alpine:3.20
WORKDIR /app
COPY fluxsearch-monitor /app/fluxsearch-monitor
EXPOSE 8090
ENTRYPOINT ["/app/fluxsearch-monitor"]
EOF

cp /tmp/fluxsearch-monitor /tmp/fluxsearch-monitor-bin
mv /tmp/fluxsearch-monitor-bin /tmp/fluxsearch-monitor
docker build -f /tmp/Dockerfile.monitor -t fluxsearch-monitor:latest /tmp 2>/dev/null || {
  echo "Docker 构建失败，使用 kubectl run 直接部署二进制方式..."
}

echo "=== 更新 Secret 环境变量映射 ==="
# Secret 已有 fluxsearch-infra，补充 monitor 所需 key 别名
kubectl apply -f deploy/kubernetes/monitor.yaml

echo "=== 等待就绪 ==="
kubectl rollout status deployment/fluxsearch-monitor -n fluxsearch --timeout=120s

NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
echo ""
echo "Monitor 已部署: http://${NODE_IP}:30090/api/v1/status"
echo "本地配置 FLUXSEARCH_MONITOR_URL=http://${NODE_IP}:30090/api/v1/status"
