#!/bin/bash
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

echo "=== Deploy PostgreSQL (raw manifest) ==="
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  namespace: fluxsearch
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 10Gi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: fluxsearch
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: bitnamilegacy/postgresql:16.4.0-debian-12-r0
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_USER
          value: fluxsearch
        - name: POSTGRES_PASSWORD
          value: changeme-in-production
        - name: POSTGRES_DB
          value: fluxsearch
        - name: PGDATA
          value: /bitnami/postgresql/data
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 1Gi
        volumeMounts:
        - name: data
          mountPath: /bitnami/postgresql
        readinessProbe:
          exec:
            command: ["pg_isready", "-U", "fluxsearch"]
          initialDelaySeconds: 15
          periodSeconds: 5
        livenessProbe:
          exec:
            command: ["pg_isready", "-U", "fluxsearch"]
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: postgres-data
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: fluxsearch
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
EOF

echo "=== Deploy Redis (raw manifest) ==="
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
        command: ["redis-server", "--requirepass", "changeme-in-production", "--appendonly", "yes"]
        resources:
          requests:
            cpu: 50m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 512Mi
        volumeMounts:
        - name: data
          mountPath: /data
        readinessProbe:
          exec:
            command: ["redis-cli", "-a", "changeme-in-production", "ping"]
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: redis-data
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: fluxsearch
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
EOF

echo "=== Wait for PG and Redis ==="
kubectl wait --for=condition=ready pod -l app=postgres -n fluxsearch --timeout=300s
kubectl wait --for=condition=ready pod -l app=redis -n fluxsearch --timeout=120s

echo "=== Deploy Milvus standalone ==="
if ! helm status milvus -n fluxsearch >/dev/null 2>&1; then
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

echo "=== Wait for Milvus (up to 5 min) ==="
sleep 15
for i in $(seq 1 30); do
  READY=$(kubectl get pods -n fluxsearch -l app.kubernetes.io/instance=milvus --no-headers 2>/dev/null | grep "Running" | wc -l)
  TOTAL=$(kubectl get pods -n fluxsearch -l app.kubernetes.io/instance=milvus --no-headers 2>/dev/null | wc -l)
  echo "Milvus attempt $i: $READY/$TOTAL running"
  if [ "$TOTAL" -gt 0 ] && [ "$READY" -eq "$TOTAL" ]; then
    break
  fi
  sleep 10
done

echo ""
echo "=== Final Status ==="
kubectl get pods,pvc,svc -n fluxsearch
helm list -n fluxsearch 2>/dev/null || true
kubectl get secret fluxsearch-infra -n fluxsearch
echo ""
echo "=== ALL DONE ==="
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FLUXSEARCH_PUBLIC_IP="${FLUXSEARCH_PUBLIC_IP:-113.128.132.69}"
echo "=== 配置外网访问 (${FLUXSEARCH_PUBLIC_IP}) ==="
bash "${SCRIPT_DIR}/apply-external-access.sh"
