#!/usr/bin/env bash
# CloudNativePG operator を入れる / 更新する。何度流しても同じ結果になる。
#
#   ./install.sh        # 適用
#   ./install.sh diff   # 何が作られるかだけ見る（クラスタには触らない）
set -euo pipefail
cd "$(dirname "$0")"

CHART_VERSION=0.29.1   # = CloudNativePG 1.30.1

helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update >/dev/null

if [[ "${1:-}" == "diff" ]]; then
  helm template cnpg cnpg/cloudnative-pg \
    --namespace cnpg-system --version "$CHART_VERSION" -f values.yaml
  exit 0
fi

helm upgrade --install cnpg cnpg/cloudnative-pg \
  --namespace cnpg-system --create-namespace \
  --version "$CHART_VERSION" -f values.yaml --wait
