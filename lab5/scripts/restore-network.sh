#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
kubectl config use-context kind-lab5-calico
helm upgrade shop "$ROOT/charts/shop" --reuse-values --wait --timeout 5m
kubectl get networkpolicy
