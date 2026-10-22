#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER_NAME="lab5-calico"
CONFIG="$ROOT/cluster/kind-calico.yaml"
LOG_DIR="$ROOT/artifacts/kind-logs"

for cmd in kind kubectl docker; do
  command -v "$cmd" >/dev/null || { echo "ERROR: $cmd is not installed" >&2; exit 1; }
done
docker info >/dev/null 2>&1 || { echo "ERROR: Docker daemon is not available" >&2; exit 1; }

if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  echo "Cluster $CLUSTER_NAME already exists; switching context."
  kubectl config use-context "kind-$CLUSTER_NAME"
  kubectl get nodes -o wide
  exit 0
fi

mkdir -p "$LOG_DIR"
echo "Creating isolated cluster $CLUSTER_NAME..."
if ! kind create cluster --name "$CLUSTER_NAME" --config "$CONFIG" --wait 5m --retain; then
  echo "Cluster creation failed. Trying to export diagnostic logs to $LOG_DIR ..." >&2
  kind export logs --name "$CLUSTER_NAME" "$LOG_DIR" >/dev/null 2>&1 || true
  echo "If no logs were exported, inspect: docker ps -a --filter name=$CLUSTER_NAME" >&2
  echo "Do not remove other kind clusters; they are unrelated to this lab." >&2
  exit 1
fi
kubectl config use-context "kind-$CLUSTER_NAME"
kubectl get nodes -o wide
