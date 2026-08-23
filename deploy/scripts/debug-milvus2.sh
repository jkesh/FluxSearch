#!/bin/bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
echo "=== Previous crash log (head) ==="
kubectl logs -n fluxsearch -l app=milvus --previous --tail=80 2>&1 | head -40
echo ""
echo "=== Current log (head) ==="
kubectl logs -n fluxsearch -l app=milvus --tail=80 2>&1 | head -40
