#!/usr/bin/env bash
# Barman Cloud Plugin を入れる / 更新する。CNPG operator と同じ cnpg-system に入れる決まり。
# 前提: cert-manager（../cert-manager/install.sh）
#   ./install.sh        # 適用
#   ./install.sh diff   # 何が作られるかだけ見る
set -euo pipefail
cd "$(dirname "$0")"

CHART_VERSION=0.8.1   # = plugin-barman-cloud v0.15.1

helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update >/dev/null

if [[ "${1:-}" == "diff" ]]; then
  helm template plugin-barman-cloud cnpg/plugin-barman-cloud \
    --namespace cnpg-system --version "$CHART_VERSION"
  exit 0
fi

helm upgrade --install plugin-barman-cloud cnpg/plugin-barman-cloud \
  --namespace cnpg-system --version "$CHART_VERSION" --wait
