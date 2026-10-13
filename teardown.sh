#!/usr/bin/env bash
# home-dbaas が k3s に足したものを全部消す。k3s 本体と kube-system の既存の部品は残す。
# DB のデータもバックアップ（Garage）も消える。元には戻せない。
#
#   ./teardown.sh          # 何を消すかの一覧だけ出す
#   ./teardown.sh --yes    # 実際に消す
#
# k3s の外に足したもの（このリポジトリ・mise のツール・gh のログイン）は README の「撤収」を見る。
set -euo pipefail

NAMESPACES=(argocd garage cnpg-system cert-manager)
# クラスタ全体に置かれるもの（CRD・権限・webhook）は名前で拾う
PATTERN='cnpg|barman|cert-manager|argocd|argoproj'

db_ns=$(kubectl get ns -l app.kubernetes.io/part-of=home-dbaas -o name)
cluster_scoped() {
  kubectl get crd,clusterrole,clusterrolebinding,mutatingwebhookconfiguration,validatingwebhookconfiguration \
    -o name | grep -E "$PATTERN" || true
}
kube_system() {
  kubectl -n kube-system get role,rolebinding,lease -o name | grep -E "$PATTERN" || true
}

echo "== DB の namespace"; echo "${db_ns:-（なし）}"
echo "== 部品の namespace"; printf '%s\n' "${NAMESPACES[@]}"
echo "== クラスタ全体のリソース"; cluster_scoped | sed 's/^/  /'
echo "== kube-system に置かれたもの"; kube_system | sed 's/^/  /'

[[ "${1:-}" == "--yes" ]] || { echo; echo "消すには --yes を付ける"; exit 0; }

# 1. Argo CD を先に止める。止めないと、消したそばから Git の状態に戻してしまう
#    （Application には finalizer を付けていないので、消しても配下のリソースは残る）
kubectl -n argocd delete applications.argoproj.io --all --ignore-not-found --wait=false || true
kubectl -n argocd scale statefulset,deployment --all --replicas=0 2>/dev/null || true

# 2. DB を消す。CNPG operator が生きているうちに消す。
#    PVC は Postgres の Pod が完全に止まるまで消えない（pvc-protection）ので、数分かかることがある
if [[ -n "$db_ns" ]]; then kubectl delete $db_ns --wait=true --timeout=600s; fi

# 3. 部品の namespace（中の Pod・PVC・Secret ごと）
kubectl delete ns "${NAMESPACES[@]}" --ignore-not-found --wait=true --timeout=600s

# 4. クラスタ全体に残るもの。CRD を消すと、その種類のリソースもすべて消える
cluster_scoped | xargs -r kubectl delete --ignore-not-found
kube_system | xargs -r kubectl -n kube-system delete --ignore-not-found

echo
echo "== 残っていないかの確認"
kubectl get ns
cluster_scoped
kubectl get pv
