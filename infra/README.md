# infra — 入れたもの

他人が作ったソフトを、バージョンを固定して入れる場所。自作のもの（`databases/`、`cmd/`）とは混ぜない。
2つ目のサービスを載せるときは、このディレクトリを丸ごと共通基盤のリポジトリへ移す。

| 名前 | バージョン | namespace | 入れ方 | 状態 |
|---|---|---|---|---|
| CloudNativePG | 1.30.1（chart 0.29.1） | `cnpg-system` | `cnpg-operator/install.sh` | 導入済み |
| cert-manager | v1.21.2 | `cert-manager` | `cert-manager/install.sh` | 導入済み（Barman Cloud Plugin の前提） |
| Barman Cloud Plugin | v0.15.1（chart 0.8.1） | `cnpg-system` | `barman-cloud/install.sh` | 導入済み |
| Garage | v2.4.1 | `garage` | `garage/install.sh`（素のマニフェスト） | 導入済み。バケット `cnpg-backups`、キー `cnpg` |
| Argo CD | v3.5.5（chart 10.10.3） | `argocd` | `../bootstrap/argocd/install.sh` | 導入済み |

バージョンを上げるときは、この表とスクリプト内の `CHART_VERSION` を同じコミットで直す。
