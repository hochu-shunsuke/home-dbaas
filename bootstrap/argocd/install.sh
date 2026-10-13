#!/usr/bin/env bash
# まっさらな k3s に home-dbaas を建てる。手で流すのはこれ1本だけ。
# Argo CD を入れて root（app-of-apps）を渡すと、あとは Argo CD が apps/ を sync-wave 順に建てる。
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
  cat ../root.yaml
  exit 0
fi

helm upgrade --install argocd argo/argo-cd \
  --namespace argocd --create-namespace \
  --version "$CHART_VERSION" -f values.yaml --wait
kubectl apply -f ../root.yaml
