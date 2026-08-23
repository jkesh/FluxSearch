#!/bin/bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
echo "=== Pods ==="
kubectl get pods -n fluxsearch -o wide 2>&1
echo "=== PVC ==="
kubectl get pvc -n fluxsearch 2>&1
echo "=== SVC ==="
kubectl get svc -n fluxsearch 2>&1
echo "=== Helm ==="
helm list -n fluxsearch 2>&1
echo "=== Events ==="
kubectl get events -n fluxsearch --sort-by='.lastTimestamp' 2>&1 | tail -15
