#!/usr/bin/env bash
set -Eeuo pipefail
CLUSTER_NAME="lab5-calico"
kubectl config use-context "kind-$CLUSTER_NAME"

# Calico v3.28.x is compatible with the Kubernetes v1.29.x node image used here.
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.28.0/manifests/calico.yaml
kubectl rollout status daemonset/calico-node -n kube-system --timeout=5m
kubectl rollout status deployment/calico-kube-controllers -n kube-system --timeout=5m
kubectl wait --for=condition=Ready nodes --all --timeout=5m
kubectl get nodes -o wide
kubectl get pods -n kube-system -o wide | grep -E 'calico|NAME'
kubectl get ippools.crd.projectcalico.org
