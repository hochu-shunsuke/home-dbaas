# このリポジトリで作業するとき

機体そのものの前提（mise・sudo・シェル設定・ネットワーク）は `~/AGENTS.md` を先に読む。ここにはこのリポジトリ固有のことだけ書く。

## 何か

hochu-ubuntu-1 の k3s 上に作る、学習用の Postgres DB クラウド。設計方針は `README.md` と `docs/decisions/`。

## 守ること

- `infra/`（入れたもの）と `databases/`・`cmd/`（作ったもの）を混ぜない（ADR 0001）
- Traefik・StorageClass・CoreDNS などクラスタ全体の設定を、このリポジトリから書き換えない
- DB の namespace には ResourceQuota と LimitRange を必ず付ける
- パスワード・鍵を Git に入れない。DB のパスワードは CNPG が生成する `<cluster>-app` Secret から読む
- 適用の前に差分を見る（`infra/*/install.sh diff`、`kubectl diff -f`）。適用したら、README の「検証」にある疎通確認をやり直す

## ツール

`helm` と `kubectl` はこのディレクトリの `mise.toml` で固定している。非対話シェルでは `mise exec -- helm ...` で呼ぶ。
`KUBECONFIG=$HOME/.kube/config` を明示する（k3s 既定の `/etc/rancher/k3s/k3s.yaml` は root 専用）。
