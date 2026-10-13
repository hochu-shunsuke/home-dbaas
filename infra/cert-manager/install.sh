#!/usr/bin/env bash
# cert-manager を入れる / 更新する。何度流しても同じ結果になる。
#   ./install.sh        # 適用
#   ./install.sh diff   # 何が作られるかだけ見る
set -euo pipefail
cd "$(dirname "$0")"

CHART_VERSION=v1.21.2

helm repo add jetstack https://charts.jetstack.io --force-update >/dev/null

if [[ "${1:-}" == "diff" ]]; then
  helm template cert-manager jetstack/cert-manager \
    --namespace cert-manager --version "$CHART_VERSION" -f values.yaml
  exit 0
fi

helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --version "$CHART_VERSION" -f values.yaml --wait
