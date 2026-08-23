#!/bin/bash
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

echo "=== Remove broken Milvus helm release ==="
helm uninstall milvus -n fluxsearch 2>/dev/null || true
kubectl delete pod -n fluxsearch -l app.kubernetes.io/instance=milvus --force --grace-period=0 2>/dev/null || true
kubectl delete pvc -n fluxsearch \
  data-milvus-etcd-0 milvus \
  milvus-pulsarv3-bookie-journal-milvus-pulsarv3-bookie-0 \
  milvus-pulsarv3-bookie-ledgers-milvus-pulsarv3-bookie-0 \
  milvus-pulsarv3-zookeeper-data-milvus-pulsarv3-zookeeper-0 \
  --ignore-not-found 2>/dev/null || true
sleep 5

echo "=== Update LimitRange for Milvus ==="
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: LimitRange
metadata:
  name: fluxsearch-limits
  namespace: fluxsearch
spec:
  limits:
  - default:
      cpu: "1"
      memory: 2Gi
    defaultRequest:
      cpu: 100m
      memory: 256Mi
    type: Container
EOF

echo "=== Deploy Milvus standalone (embedded etcd, external MinIO) ==="
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: milvus-data
  namespace: fluxsearch
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 20Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: milvus
  namespace: fluxsearch
  labels:
    app: milvus
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: milvus
  template:
    metadata:
      labels:
        app: milvus
    spec:
      containers:
      - name: milvus
        image: milvusdb/milvus:v2.6.21
        args: ["milvus", "run", "standalone"]
        env:
        - name: ETCD_USE_EMBED
          value: "true"
        - name: ETCD_DATA_DIR
          value: /var/lib/milvus/etcd
        - name: COMMON_STORAGETYPE
          value: minio
        - name: MINIO_ADDRESS
          value: minio.fluxsearch.svc.cluster.local:9000
        - name: MINIO_ACCESS_KEY_ID
          value: fluxsearch
        - name: MINIO_SECRET_ACCESS_KEY
          value: changeme-in-production
        - name: MINIO_BUCKET_NAME
          value: milvus-bucket
        - name: MINIO_USE_SSL
          value: "false"
        - name: MINIO_REGION
          value: us-east-1
        ports:
        - name: grpc
          containerPort: 19530
        - name: metrics
          containerPort: 9091
        resources:
          requests:
            cpu: 500m
            memory: 2Gi
          limits:
            cpu: 2000m
            memory: 4Gi
        readinessProbe:
          httpGet:
            path: /healthz
            port: 9091
          initialDelaySeconds: 60
          periodSeconds: 15
          timeoutSeconds: 5
        livenessProbe:
          httpGet:
            path: /healthz
            port: 9091
          initialDelaySeconds: 90
          periodSeconds: 30
          timeoutSeconds: 5
        volumeMounts:
        - name: data
          mountPath: /var/lib/milvus
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: milvus-data
---
apiVersion: v1
kind: Service
metadata:
  name: milvus
  namespace: fluxsearch
spec:
  selector:
    app: milvus
  ports:
  - name: grpc
    port: 19530
    targetPort: 19530
  - name: metrics
    port: 9091
    targetPort: 9091
EOF

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

echo "=== Wait for Milvus (up to 3 min) ==="
for i in $(seq 1 18); do
  STATUS=$(kubectl get pod -n fluxsearch -l app=milvus -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
  READY=$(kubectl get pod -n fluxsearch -l app=milvus -o jsonpath='{.items[0].status.containerStatuses[0].ready}' 2>/dev/null || echo "false")
  echo "Attempt $i: phase=$STATUS ready=$READY"
  if [ "$READY" = "true" ]; then break; fi
  sleep 10
done

echo ""
echo "=== Final Status ==="
kubectl get pods,pvc,svc,secret -n fluxsearch
echo ""
echo "=== Connection Info ==="
echo "PostgreSQL: postgres.fluxsearch.svc:5432  user=fluxsearch  db=fluxsearch"
echo "Redis:      redis.fluxsearch.svc:6379"
echo "MinIO:      minio.fluxsearch.svc:9000  bucket=milvus-bucket"
echo "Milvus:     milvus.fluxsearch.svc:19530"
echo "Secret:     fluxsearch-infra (namespace: fluxsearch)"
echo "=== DONE ==="
