#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if command -v k6 >/dev/null; then
  k6 run "$ROOT/load/order.js"
else
  echo "k6 is not installed; run it in Docker instead:"
  echo "docker run --rm --network host -v "$ROOT/load:/scripts:ro" grafana/k6 run /scripts/order.js"
  exit 2
fi
