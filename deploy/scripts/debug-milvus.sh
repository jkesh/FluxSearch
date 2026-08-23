#!/bin/bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
echo "=== Milvus logs ==="
kubectl logs -n fluxsearch -l app=milvus --tail=50 2>&1
echo ""
echo "=== Milvus describe ==="
kubectl describe pod -n fluxsearch -l app=milvus 2>&1 | tail -30
echo ""
echo "=== MinIO bucket check ==="
kubectl run minio-check --rm -i --restart=Never -n fluxsearch \
  --image=minio/mc:latest \
  --overrides='{"spec":{"containers":[{"name":"minio-check","image":"minio/mc:latest","command":["/bin/sh","-c","mc alias set local http://minio.fluxsearch.svc.cluster.local:9000 fluxsearch changeme-in-production && mc ls local/ && mc ls local/milvus-bucket/"],"resources":{"requests":{"cpu":"50m","memory":"64Mi"},"limits":{"cpu":"200m","memory":"256Mi"}}}]}}' 2>&1 || true
