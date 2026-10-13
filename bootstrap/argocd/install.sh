#!/usr/bin/env bash
# Argo CD を入れる / 更新する。ここだけは手で流す。以降の変更は Git 経由。
#
#   ./install.sh        # 適用
#   ./install.sh diff   # 何が作られるかだけ見る（クラスタには触らない）
set -euo pipefail
cd "$(dirname "$0")"

CHART_VERSION=10.10.3   # = Argo CD v3.5.5

helm repo add argo https://argoproj.github.io/argo-helm --force-update >/dev/null

if [[ "${1:-}" == "diff" ]]; then
  helm template argocd argo/argo-cd \
    --namespace argocd --version "$CHART_VERSION" -f values.yaml
  cat ingress.yaml ../databases.yaml
  exit 0
fi

helm upgrade --install argocd argo/argo-cd \
  --namespace argocd --create-namespace \
  --version "$CHART_VERSION" -f values.yaml --wait
kubectl apply -f ingress.yaml
kubectl apply -f ../databases.yaml
