# このリポジトリで作業するとき

機体そのものの前提（mise・sudo・シェル設定・ネットワーク）は `~/AGENTS.md` を先に読む。ここにはこのリポジトリ固有のことだけ書く。

## 何か

hochu-ubuntu-1 の k3s 上に作る、学習用の Postgres DB クラウド。設計方針は `README.md` と `docs/decisions/`。

## 守ること

- `infra/`（入れたもの）と `databases/`・`cmd/`（作ったもの）を混ぜない（ADR 0001）
- Traefik・StorageClass・CoreDNS などクラスタ全体の設定を、このリポジトリから書き換えない
- DB の namespace には ResourceQuota と LimitRange を必ず付ける
- パスワード・鍵を Git に入れない。DB のパスワードは CNPG が生成する `<cluster>-app` Secret から読む
- クラスタへの変更は Git 経由（`apps/`・`infra/`・`databases/` を直して push）。手で `kubectl apply` しない
- push の前に差分を見る（`kubectl diff -f`、helm chart は `helm template`）。反映後、README の「検証」にある疎通確認をやり直す
- 秘密はクラスタの中で生成する（`infra/garage/bootstrap.yaml` が例）。秘密を表示・記録するコマンドを流さない

## ツール

`helm` と `kubectl` はこのディレクトリの `mise.toml` で固定している。非対話シェルでは `mise exec -- helm ...` で呼ぶ。
`KUBECONFIG=$HOME/.kube/config` を明示する（k3s 既定の `/etc/rancher/k3s/k3s.yaml` は root 専用）。
