#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="observability-demo"
CONFIG="$(dirname "$0")/kind-config.yaml"

# Проверки
command -v kind >/dev/null    || { echo "kind не установлен"; exit 1; }
command -v kubectl >/dev/null || { echo "kubectl не установлен"; exit 1; }
docker info >/dev/null 2>&1   || { echo "Docker-демон недоступен"; exit 1; }

# Идемпотентность: если кластер есть — выходим
if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  echo "Кластер '$CLUSTER_NAME' уже существует"
  kubectl config use-context "kind-$CLUSTER_NAME"
  kubectl get nodes
  exit 0
fi

# Создание
echo "Создаю кластер '$CLUSTER_NAME'..."
kind create cluster --config "$CONFIG" --wait 120s

# Проверка готовности
kubectl wait --for=condition=Ready nodes --all --timeout=120s
kubectl get nodes -o wide