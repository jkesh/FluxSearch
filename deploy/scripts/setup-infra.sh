#!/bin/bash
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

echo "=== Step 1: Setup kubeconfig for deploy ==="
mkdir -p /home/deploy/.kube
cp /etc/rancher/k3s/k3s.yaml /home/deploy/.kube/config
chown -R deploy:deploy /home/deploy/.kube
sed -i 's/127.0.0.1/your-k8s-node.example.com/g' /home/deploy/.kube/config

echo "=== Step 2: Create namespace ==="
kubectl create namespace fluxsearch --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ResourceQuota
metadata:
  name: fluxsearch-quota
  namespace: fluxsearch
spec:
  hard:
    requests.cpu: "3"
    requests.memory: 12Gi
    limits.cpu: "5"
    limits.memory: 20Gi
    persistentvolumeclaims: "8"
EOF

echo "=== Step 3: Add Helm repos ==="
helm repo add minio https://charts.min.io/ 2>/dev/null || true
helm repo add bitnami https://charts.bitnami.com/bitnami 2>/dev/null || true
helm repo add milvus https://zilliztech.github.io/milvus-helm 2>/dev/null || true
helm repo update

echo "=== Step 4: Deploy MinIO ==="
if ! helm status minio -n fluxsearch >/dev/null 2>&1; then
  helm install minio minio/minio -n fluxsearch \
    --set mode=standalone \
    --set replicas=1 \
    --set persistence.size=30Gi \
    --set resources.requests.memory=256Mi \
    --set resources.requests.cpu=100m \
    --set resources.limits.memory=1Gi \
    --set resources.limits.cpu=500m \
    --set rootUser=fluxsearch \
    --set rootPassword=changeme-in-production \
    --set buckets[0].name=milvus-bucket \
    --set buckets[0].policy=none \
    --set buckets[0].purge=false
else
  echo "MinIO already installed, skipping"
fi

echo "=== Step 5: Deploy PostgreSQL ==="
if ! helm status fluxsearch-pg -n fluxsearch >/dev/null 2>&1; then
  helm install fluxsearch-pg bitnami/postgresql -n fluxsearch \
    --set auth.username=fluxsearch \
    --set auth.password=changeme-in-production \
    --set auth.database=fluxsearch \
    --set primary.persistence.size=10Gi \
    --set primary.resources.requests.memory=256Mi \
    --set primary.resources.requests.cpu=100m \
    --set primary.resources.limits.memory=1Gi \
    --set primary.resources.limits.cpu=500m \
    --set image.repository=bitnamilegacy/postgresql \
    --set image.tag=16.4.0-debian-12-r0
else
  echo "PostgreSQL already installed, skipping"
fi

echo "=== Step 6: Deploy Redis ==="
if ! helm status fluxsearch-redis -n fluxsearch >/dev/null 2>&1; then
  helm install fluxsearch-redis bitnami/redis -n fluxsearch \
    --set architecture=standalone \
    --set auth.enabled=true \
    --set auth.password=changeme-in-production \
    --set master.persistence.size=2Gi \
    --set master.resources.requests.memory=128Mi \
    --set master.resources.requests.cpu=50m \
    --set master.resources.limits.memory=512Mi \
    --set master.resources.limits.cpu=200m \
    --set image.repository=bitnamilegacy/redis \
    --set image.tag=7.4.1-debian-12-r0
else
  echo "Redis already installed, skipping"
fi

echo "=== Step 7: Wait for MinIO ready ==="
kubectl wait --for=condition=ready pod -l app=minio -n fluxsearch --timeout=180s

echo "=== Step 8: Deploy Milvus standalone ==="
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
  echo "Milvus already installed, skipping"
fi

echo "=== Step 9: Create connection secret ==="
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Secret
metadata:
  name: fluxsearch-infra
  namespace: fluxsearch
type: Opaque
stringData:
  POSTGRES_HOST: fluxsearch-pg-postgresql.fluxsearch.svc.cluster.local
  POSTGRES_PORT: "5432"
  POSTGRES_USER: fluxsearch
  POSTGRES_PASSWORD: changeme-in-production
  POSTGRES_DB: fluxsearch
  REDIS_HOST: fluxsearch-redis-master.fluxsearch.svc.cluster.local
  REDIS_PORT: "6379"
  REDIS_PASSWORD: changeme-in-production
  MINIO_ENDPOINT: minio.fluxsearch.svc.cluster.local:9000
  MINIO_ACCESS_KEY: fluxsearch
  MINIO_SECRET_KEY: changeme-in-production
  MINIO_BUCKET: milvus-bucket
  MILVUS_HOST: milvus.fluxsearch.svc.cluster.local
  MILVUS_PORT: "19530"
EOF

echo "=== Step 10: Status ==="
kubectl get pods,pvc,svc -n fluxsearch
echo ""
echo "=== Deployment complete ==="
