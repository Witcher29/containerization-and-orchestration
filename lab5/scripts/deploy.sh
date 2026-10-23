#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
kubectl config use-context kind-lab5-calico
kubectl wait --for=condition=Ready nodes --all --timeout=5m

# Install the same CloudNativePG operator and observability stack used in Lab 4.
kubectl apply --server-side -f "$ROOT/operator-cloudnative-pg.yaml"
kubectl wait --for=condition=Available deployment/cnpg-controller-manager -n cnpg-system --timeout=5m
helm upgrade --install observability "$ROOT/observability" --namespace monitoring --create-namespace --wait --timeout 15m

# Build and load application images into the dedicated lab5 cluster.
docker build -t shop-api:v1 "$ROOT/service/api"
docker build -t shop-worker:v1 "$ROOT/service/worker"
docker build -t shop-batch:v1 "$ROOT/service/batch"
kind load docker-image shop-api:v1 --name lab5-calico
kind load docker-image shop-worker:v1 --name lab5-calico
kind load docker-image shop-batch:v1 --name lab5-calico

helm upgrade --install shop "$ROOT/charts/shop" --namespace default --wait --timeout 10m
kubectl get pods,svc,networkpolicy
