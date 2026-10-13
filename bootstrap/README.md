# bootstrap

手で流すのはここだけ。`argocd/install.sh` が Argo CD を helm で入れ、`root.yaml` を apply する。
以降の変更はすべて Git → Argo CD 経由。

| ファイル | 役割 |
|---|---|
| `argocd/install.sh` | Argo CD v3.5.5（chart 10.10.3）を入れ、`root.yaml` を apply する |
| `argocd/values.yaml` | HA なし・Dex なし・通知なし・`/argocd` 配下で配信。子 Application の健康判定を有効化 |
| `argocd/ingress.yaml` | Traefik の :80 に `/argocd` を足す（`apps/argocd-ingress.yaml` が入れる） |
| `root.yaml` | app-of-apps の根。`apps/` の Application を sync-wave 順に建てる |

Argo CD 自身は Argo CD で管理していない（鶏と卵になるため、helm で入れる1段だけ手作業に残している）。
