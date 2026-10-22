#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
command -v helm >/dev/null || { echo "helm not installed; skipped helm template"; exit 0; }
helm lint "$ROOT/charts/shop"
helm template shop "$ROOT/charts/shop" > "$ROOT/artifacts-rendered.yaml"
echo "Rendered manifests to $ROOT/artifacts-rendered.yaml"
