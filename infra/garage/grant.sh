#!/usr/bin/env bash
# DB の namespace に、バックアップ用 S3 キーの Secret（backup-s3）を置く。
# Git に鍵を入れないため、ここだけ手で流す（秘密の GitOps 化はロードマップ 5 で考える）。
#   ./grant.sh db-sandbox
set -euo pipefail
ns=${1:?usage: grant.sh <namespace>}

info=$(kubectl -n garage exec garage-0 -- /garage key info cnpg --show-secret)
id=$(sed -n "s/^Key ID: *//p" <<<"$info")
secret=$(sed -n "s/^Secret key: *//p" <<<"$info")
[[ -n "$id" && -n "$secret" ]] || { echo "キーが読めない。garage key info の出力形式を確認する" >&2; exit 1; }

kubectl -n "$ns" create secret generic backup-s3 \
  --from-literal=ACCESS_KEY_ID="$id" --from-literal=ACCESS_SECRET_KEY="$secret" \
  --dry-run=client -o yaml | kubectl apply -f -
