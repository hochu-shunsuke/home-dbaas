#!/usr/bin/env bash
# Garage を建て、バックアップ用のバケットとキーを用意する。何度流しても同じ結果になる。
#   ./install.sh        # 適用
#   ./install.sh diff   # kubectl diff で差分だけ見る
set -euo pipefail
cd "$(dirname "$0")"

BUCKET=cnpg-backups
KEY=cnpg
CAPACITY=20G

if [[ "${1:-}" == "diff" ]]; then
  kubectl diff -f garage.yaml || true
  exit 0
fi

kubectl apply -f garage.yaml

# 秘密は初回だけ生成する。作り直すと既存のデータが読めなくなる
if ! kubectl -n garage get secret garage-secrets >/dev/null 2>&1; then
  kubectl -n garage create secret generic garage-secrets \
    --from-literal=rpc_secret="$(openssl rand -hex 32)" \
    --from-literal=admin_token="$(openssl rand -base64 32)"
fi

kubectl -n garage rollout status statefulset/garage --timeout=180s
g() { kubectl -n garage exec garage-0 -- /garage "$@"; }

# ノードに容量を割り当てる（レイアウト）。割り当て済みなら飛ばす
if ! g layout show 2>/dev/null | grep -qE "^[0-9a-f]{16} .* home "; then
  node=$(g node id -q | cut -d@ -f1)
  g layout assign -z home -c "$CAPACITY" "$node"
  ver=$(g layout show | sed -n "s/.*garage layout apply --version \([0-9]*\).*/\1/p")
  g layout apply --version "$ver"
fi

g bucket info "$BUCKET" >/dev/null 2>&1 || g bucket create "$BUCKET"
g key info "$KEY" >/dev/null 2>&1 || g key create "$KEY" >/dev/null  # 出力に秘密鍵が含まれるので捨てる
g bucket allow --read --write --owner "$BUCKET" --key "$KEY" >/dev/null
g status 2>/dev/null
