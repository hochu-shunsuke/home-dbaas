# infra — 入れたもの

他人が作ったソフトの設定を置く場所。入れるのは Argo CD（`apps/` の Application）で、ここはその中身。
自作のもの（`databases/`、`cmd/`）とは混ぜない。2つ目のサービスを載せるときは、
`apps/` と `infra/` を丸ごと共通基盤のリポジトリへ移す。

| 名前 | バージョン | namespace | Application | 設定 |
|---|---|---|---|---|
| CloudNativePG | 1.30.1（chart 0.29.1） | `cnpg-system` | `apps/cnpg-operator.yaml` | `cnpg-operator/values.yaml` |
| cert-manager | v1.21.2 | `cert-manager` | `apps/cert-manager.yaml` | `cert-manager/values.yaml` |
| Barman Cloud Plugin | v0.15.1（chart 0.8.1） | `cnpg-system` | `apps/barman-cloud.yaml` | chart の既定値のまま |
| Garage | v2.4.1 | `garage` | `apps/garage.yaml` | `garage/`（素のマニフェスト + 初期化 Job） |
| Argo CD | v3.5.5（chart 10.10.3） | `argocd` | — （`bootstrap/argocd/install.sh` が helm で入れる） | `../bootstrap/argocd/values.yaml` |

バージョンを上げるときは、この表と Application の `targetRevision` を同じコミットで直す。
