# bootstrap

ここだけは手で1回流す。それ以降の変更はすべて Git → Argo CD 経由。

| ファイル | 役割 |
|---|---|
| `argocd/install.sh` | Argo CD v3.5.5（chart 10.10.3）を入れ、下の2枚を apply する |
| `argocd/values.yaml` | HA なし・Dex なし・通知なし・`/argocd` 配下で配信 |
| `argocd/ingress.yaml` | Traefik の :80 に `/argocd` を足す（host 指定なし） |
| `databases.yaml` | `databases/` を見張る Application。`prune` と `selfHeal` を有効化 |

`prune: true` なので、`databases/` からファイルを消して push すると **DB がデータごと消える**。
段階3でバックアップが入るまでは、大事なデータを置かない。
