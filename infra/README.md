# infra — 入れたもの

他人が作ったソフトを、バージョンを固定して入れる場所。自作のもの（`databases/`、`cmd/`）とは混ぜない。
2つ目のサービスを載せるときは、このディレクトリを丸ごと共通基盤のリポジトリへ移す。

| 名前 | バージョン | namespace | 入れ方 | 状態 |
|---|---|---|---|---|
| CloudNativePG | 1.30.1（chart 0.29.1） | `cnpg-system` | `cnpg-operator/install.sh` | 導入済み |
| cert-manager | — | — | — | 段階3で入れる（Barman Cloud Plugin の前提） |
| Barman Cloud Plugin | v0.15.1 予定 | `cnpg-system` | — | 段階3 |
| Garage | — | — | — | 段階3（S3 互換。MinIO は使わない → ADR 0003） |
| Argo CD | v3.5.x 予定 | `argocd` | — | 段階2（`bootstrap/`） |

バージョンを上げるときは、この表とスクリプト内の `CHART_VERSION` を同じコミットで直す。
