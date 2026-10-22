#!/usr/bin/env bash
set -Eeuo pipefail
kubectl config use-context kind-lab5-calico

cleanup() {
  kubectl delete pod lab5-api-probe lab5-worker-probe lab5-denied-probe lab5-client --ignore-not-found --wait=false >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Probe pods use network-role rather than app selectors, so they do not become endpoints of shop Services.
for spec in "lab5-api-probe:api" "lab5-worker-probe:worker" "lab5-denied-probe:denied" "lab5-client:api-client"; do
  name="${spec%%:*}"; role="${spec##*:}"
  kubectl run "$name" --image=busybox:1.36 --restart=Never     --labels="network-role=$role,lab5-probe=true" --command -- sh -c 'sleep 3600' >/dev/null
 done
for p in lab5-api-probe lab5-worker-probe lab5-denied-probe lab5-client; do
  kubectl wait --for=condition=Ready "pod/$p" --timeout=120s
done

check_allowed() {
  local pod="$1" host="$2" port="$3"
  echo "EXPECT ALLOWED: $pod -> $host:$port"
  kubectl exec "$pod" -- nc -z -w 3 "$host" "$port"
}
check_denied() {
  local pod="$1" host="$2" port="$3"
  echo "EXPECT DENIED: $pod -> $host:$port"
  if kubectl exec "$pod" -- nc -z -w 3 "$host" "$port"; then
    echo "ERROR: connection unexpectedly succeeded" >&2
    return 1
  else
    echo "OK: connection blocked as expected"
  fi
}

check_allowed lab5-api-probe shop-postgres-rw 5432
check_allowed lab5-worker-probe shop-postgres-rw 5432
check_denied lab5-denied-probe shop-postgres-rw 5432
check_allowed lab5-client shop-api 80
check_denied lab5-denied-probe shop-api 80
