#!/bin/bash
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

echo "=== Remove broken Milvus ==="
kubectl delete deployment milvus -n fluxsearch --ignore-not-found
kubectl delete svc milvus -n fluxsearch --ignore-not-found
sleep 3

echo "=== Deploy etcd ==="
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: etcd-data
  namespace: fluxsearch
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: etcd
  namespace: fluxsearch
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: etcd
  template:
    metadata:
      labels:
        app: etcd
    spec:
      containers:
      - name: etcd
        image: quay.io/coreos/etcd:v3.5.16
        command:
        - etcd
        - --advertise-client-urls=http://0.0.0.0:2379
        - --listen-client-urls=http://0.0.0.0:2379
        - --data-dir=/etcd-data
        ports:
        - containerPort: 2379
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
        volumeMounts:
        - name: data
          mountPath: /etcd-data
        readinessProbe:
          exec:
            command: ["etcdctl", "endpoint", "health"]
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: etcd-data
---
apiVersion: v1
kind: Service
metadata:
  name: etcd
  namespace: fluxsearch
spec:
  selector:
    app: etcd
  ports:
  - port: 2379
    targetPort: 2379
EOF

echo "=== Wait for etcd ==="
kubectl wait --for=condition=ready pod -l app=etcd -n fluxsearch --timeout=120s

echo "=== Deploy Milvus standalone (external etcd + minio) ==="
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: milvus
  namespace: fluxsearch
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
        image: milvusdb/milvus:v2.4.17
        args: ["milvus", "run", "standalone"]
        env:
        - name: ETCD_ENDPOINTS
          value: etcd.fluxsearch.svc.cluster.local:2379
        - name: MINIO_ADDRESS
          value: minio.fluxsearch.svc.cluster.local:9000
        - name: MINIO_ACCESS_KEY_ID
          value: fluxsearch
        - name: MINIO_SECRET_ACCESS_KEY
          value: changeme-in-production
        ports:
        - name: grpc
          containerPort: 19530
        - name: metrics
          containerPort: 9091
        resources:
          requests:
            cpu: 300m
            memory: 1536Mi
          limits:
            cpu: 1500m
            memory: 3Gi
        readinessProbe:
          httpGet:
            path: /healthz
            port: 9091
          initialDelaySeconds: 90
          periodSeconds: 15
          failureThreshold: 6
        livenessProbe:
          httpGet:
            path: /healthz
            port: 9091
          initialDelaySeconds: 120
          periodSeconds: 30
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

echo "=== Wait for Milvus (up to 4 min) ==="
for i in $(seq 1 24); do
  READY=$(kubectl get pod -n fluxsearch -l app=milvus -o jsonpath='{.items[0].status.containerStatuses[0].ready}' 2>/dev/null || echo "false")
  PHASE=$(kubectl get pod -n fluxsearch -l app=milvus -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
  echo "Attempt $i: phase=$PHASE ready=$READY"
  if [ "$READY" = "true" ]; then break; fi
  sleep 10
done

echo ""
echo "=== Final Status ==="
kubectl get pods,pvc,svc -n fluxsearch
echo "=== DONE ==="
