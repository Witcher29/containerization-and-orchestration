#!/usr/bin/env bash
set -Eeuo pipefail
CLUSTER_NAME="lab5-calico"
if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME"
else
  echo "Cluster $CLUSTER_NAME does not exist."
fi
