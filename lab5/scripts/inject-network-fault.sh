#!/usr/bin/env bash
set -Eeuo pipefail
kubectl config use-context kind-lab5-calico
# Removing this allow policy makes API egress default-deny; this intentionally breaks API -> PostgreSQL.
kubectl delete networkpolicy shop-api-egress
cat <<'EOF'
Fault injected: API no longer has an egress allow rule to PostgreSQL.
Wait ~30 seconds, then send POST requests to http://localhost:30080/order and inspect:
  kubectl logs deploy/shop-api --since=2m
  kubectl port-forward -n monitoring svc/observability-kube-prometh-prometheus 9090:9090
Prometheus expression: sum(rate(http_requests_total{code=~"5.."}[1m]))
Restore immediately after capturing evidence:
  helm upgrade shop ./charts/shop --reuse-values --wait --timeout 5m
EOF
