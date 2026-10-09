#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="observability-demo"

if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME"
  echo "Кластер удалён"
else
  echo "Кластера '$CLUSTER_NAME' нет"
fi