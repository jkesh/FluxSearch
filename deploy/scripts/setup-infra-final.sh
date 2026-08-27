#!/bin/bash
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# 通过 SSH 隧道使用本地 v2rayN 代理 (socks5://127.0.0.1:11080)
export ALL_PROXY=socks5://127.0.0.1:11080
export HTTP_PROXY=socks5://127.0.0.1:11080
export HTTPS_PROXY=socks5://127.0.0.1:11080

echo "=== Fix Redis (reset PVC + bitnami entrypoint) ==="
kubectl delete statefulset redis -n fluxsearch --ignore-not-found
kubectl delete pod redis-0 -n fluxsearch --ignore-not-found
kubectl delete pvc redis-data -n fluxsearch --ignore-not-found
sleep 5

kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-data
  namespace: fluxsearch
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 2Gi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: fluxsearch
spec:
  serviceName: redis
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: bitnamilegacy/redis:7.4.1-debian-12-r0
        ports:
        - containerPort: 6379
        env:
        - name: REDIS_PASSWORD
          value: changeme-in-production
        - name: REDIS_AOF_ENABLED
          value: "yes"
        resources:
          requests:
            cpu: 50m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 512Mi
        volumeMounts:
        - name: data
          mountPath: /bitnami/redis/data
        readinessProbe:
          exec:
            command: ["sh", "-c", "redis-cli -a $REDIS_PASSWORD ping | grep PONG"]
          initialDelaySeconds: 10
          periodSeconds: 5
        livenessProbe:
          exec:
            command: ["sh", "-c", "redis-cli -a $REDIS_PASSWORD ping | grep PONG"]
          initialDelaySeconds: 20
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: redis-data
EOF

echo "=== Wait for Redis ==="
kubectl wait --for=condition=ready pod -l app=redis -n fluxsearch --timeout=180s

echo "=== Test proxy connectivity ==="
curl -s --proxy socks5://127.0.0.1:11080 --max-time 10 https://registry-1.docker.io/v2/ && echo "proxy OK" || echo "proxy failed, will try helm anyway"

echo "=== Deploy Milvus standalone ==="
if ! helm status milvus -n fluxsearch >/dev/null 2>&1; then
  helm repo update milvus 2>/dev/null || helm repo add milvus https://zilliztech.github.io/milvus-helm
  helm install milvus milvus/milvus -n fluxsearch \
    --set cluster.enabled=false \
    --set etcd.replicaCount=1 \
    --set etcd.resources.requests.cpu=100m \
    --set etcd.resources.requests.memory=256Mi \
    --set etcd.resources.limits.cpu=500m \
    --set etcd.resources.limits.memory=1Gi \
    --set minio.enabled=false \
    --set externalS3.enabled=true \
    --set externalS3.host=minio.fluxsearch.svc.cluster.local \
    --set externalS3.port=9000 \
    --set externalS3.accessKey=fluxsearch \
    --set externalS3.secretKey=changeme-in-production \
    --set externalS3.bucketName=milvus-bucket \
    --set externalS3.useSSL=false \
    --set pulsar.enabled=false \
    --set standalone.resources.requests.memory=1Gi \
    --set standalone.resources.requests.cpu=300m \
    --set standalone.resources.limits.memory=3Gi \
    --set standalone.resources.limits.cpu=1500m \
    --set standalone.persistence.enabled=true \
    --set standalone.persistence.size=20Gi
else
  echo "Milvus already installed"
fi

echo "=== Create connection secret ==="
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Secret
metadata:
  name: fluxsearch-infra
  namespace: fluxsearch
type: Opaque
stringData:
  POSTGRES_HOST: postgres.fluxsearch.svc.cluster.local
  POSTGRES_PORT: "5432"
  POSTGRES_USER: fluxsearch
  POSTGRES_PASSWORD: changeme-in-production
  POSTGRES_DB: fluxsearch
  REDIS_HOST: redis.fluxsearch.svc.cluster.local
  REDIS_PORT: "6379"
  REDIS_PASSWORD: changeme-in-production
  MINIO_ENDPOINT: minio.fluxsearch.svc.cluster.local:9000
  MINIO_ACCESS_KEY: fluxsearch
  MINIO_SECRET_KEY: changeme-in-production
  MINIO_BUCKET: milvus-bucket
  MILVUS_HOST: milvus.fluxsearch.svc.cluster.local
  MILVUS_PORT: "19530"
EOF

echo "=== Wait for Milvus pods ==="
sleep 20
for i in $(seq 1 36); do
  TOTAL=$(kubectl get pods -n fluxsearch --no-headers 2>/dev/null | grep -v Completed | wc -l)
  READY=$(kubectl get pods -n fluxsearch --no-headers 2>/dev/null | grep "1/1.*Running\|2/2.*Running" | wc -l)
  echo "Attempt $i: $READY/$TOTAL ready"
  NOT_READY=$(kubectl get pods -n fluxsearch --no-headers 2>/dev/null | grep -v "Running\|Completed" | wc -l)
  if [ "$NOT_READY" -eq 0 ] && [ "$TOTAL" -gt 0 ]; then
  MILVUS=$(kubectl get pods -n fluxsearch -l app.kubernetes.io/instance=milvus --no-headers 2>/dev/null | grep Running | wc -l)
  if [ "$MILVUS" -gt 0 ]; then break; fi
  fi
  sleep 10
done

echo ""
echo "=== Final Status ==="
kubectl get pods,pvc,svc -n fluxsearch
helm list -n fluxsearch 2>/dev/null || true
kubectl get secret fluxsearch-infra -n fluxsearch 2>/dev/null
echo "=== DONE ==="
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FLUXSEARCH_PUBLIC_IP="${FLUXSEARCH_PUBLIC_IP:-113.128.132.69}"
bash "${SCRIPT_DIR}/apply-external-access.sh"
