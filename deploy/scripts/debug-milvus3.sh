#!/bin/bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl logs -n fluxsearch -l app=milvus --previous 2>&1 | grep -iE 'error|fatal|panic|fail|FATAL|ERROR' | head -20
echo "---"
kubectl describe pod -n fluxsearch -l app=milvus 2>&1 | grep -iE 'OOM|Exit|Reason|State|Last State' | head -15
echo "---"
kubectl top nodes 2>/dev/null
kubectl top pods -n fluxsearch 2>/dev/null
