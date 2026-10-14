# 作業ログと再開手順

最終更新: 2026-10-14（段階5の途中で中断）

設計方針は [README](../README.md) と [ADR](decisions/)。ここは「どこまで進んだか」と「次に何をするか」だけ。

## 前提（変えないこと）

- 学習のための一回きりの構築。常設ではない。完全なバックアップや災害対策は目指さない
- 影響は hochu-ubuntu-1 の中に閉じる。クラウドのサービスは使わない（GitHub はリポジトリ置き場、GHCR はイメージ置き場としてだけ使う）
- 秘密はクラスタの中で生成し、Git にもログにも出さない
- 汎用基盤ではなく、このサービス専用の構成（ADR 0001）

## 作業環境

| もの | 場所・入り方 |
|---|---|
| 作業場所 | Ubuntu の `~/src/home-dbaas`（`main` を追跡、`gh` ログイン済み・push 可） |
| 入り方 | 自宅は `ssh ubuntu-lan`、外出先は `ssh ubuntu-ts`（Mac の Tailscale を起動しておく） |
| ツール | `mise.toml`（helm 4.3.0 / kubectl 1.36.4 / kubebuilder 4.16.0）。Go 1.27.1 は mise のグローバル |
| 非対話シェル | `bash -lc "..."` で包む。`KUBECONFIG=~/.kube/config` を明示 |
| Argo CD の画面 | `http://192.168.0.240/argocd`（外出先は `http://100.78.118.56/argocd`） |
| CI | `.github/workflows/operator.yaml`。`operator/` を push するとイメージを作り、タグを書き戻す |

## 段階ごとの状態

| 段階 | 状態 |
|---|---|
| 1 CNPG で Postgres | 完了 |
| 2 Argo CD で GitOps | 完了 |
| 3 バックアップと PITR | 完了 |
| 4 app-of-apps・秘密の生成・撤収 → 再構築の総合試験 | 完了 |
| **5 自作 Operator** | **途中**（下記） |
| 6 ブランチ・SNI の入口 | 未着手 |
| 7 観測（任意） | 未着手 |

## 段階5: どこまで進んだか

済んだこと:

- `operator/` に kubebuilder の雛形と実装（`Database` CRD、Reconcile、Garage クライアント）
- 開発モード（Ubuntu で `bin/manager` を直接実行）で確認済み:
  - `Database` を置いて約30秒で Ready。専用バケット `db-<name>-<uid8>` と専用キーを発行
  - テナント分離: 自分のバケットは 200、他人のバケットは 403 AccessDenied
  - 削除: finalizer が namespace → バケット（S3 API で空にして削除）→ キーの順に片付け、約40秒
  - status の書き込みを Update から MergeFrom の Patch に変え、競合エラーが 0 件に
- CI が 1 回目で成功。イメージ `ghcr.io/hochu-shunsuke/home-dbaas-operator:sha-0fd9594` は匿名で pull できる（公開設定は不要だった）
- `apps/operator.yaml`（wave 1）で Argo CD が Operator を `dbaas-system` に配備し、Running を確認

**途中で中断した作業**（`sandbox` を手書きの YAML から `Database` へ移す）:

1. `921fe00` で `databases/sandbox.yaml` を削除した。最後の YAML だったため `databases/` ごと Git から消え、
   Argo CD の `databases` アプリが `app path does not exist` で止まった（安全側に倒れて何も消さなかった）
2. `dc285a1` で `databases/README.md` を足してディレクトリを戻した。**このコマンドはユーザーが止めたが、push までは実行されていた**
3. そのため、Argo CD が次に Git を読むと、古い `sandbox`（namespace `db-sandbox`）を prune する。
   **中断時点では Ubuntu に届かず、消えたかどうかは未確認**

## 再開したら最初にやること

```bash
ssh ubuntu-lan    # 外出先なら Mac の Tailscale を起動して ubuntu-ts
cd ~/src/home-dbaas && git pull --ff-only
export KUBECONFIG=~/.kube/config

kubectl -n argocd get applications              # 7 + operator が Synced / Healthy か
kubectl get ns db-sandbox                       # 古い sandbox が prune されたか（NotFound なら消えた）
kubectl -n dbaas-system get pods                # Operator が Running か
kubectl get databases                           # まだ 0 件のはず
```

古い `sandbox` が消えていたら、次の段階5の続きへ。残っていたら、Argo CD の `databases` アプリの
conditions を見る（`kubectl -n argocd get application databases -o yaml`）。

## 段階5の残り

1. `databases/sandbox.yaml` を `Database` 1枚（`plan: small`）で置き直して push。Ready・接続・バックアップを確認
2. 不要になった手作業を消す
   - `infra/garage/grant.sh` を削除
   - `infra/garage/bootstrap.yaml` の `setup.py` から、共有バケット `cnpg-backups` とキー `cnpg` の作成を外す（レイアウトだけ残す）
   - 既に作られた `cnpg-backups` と `cnpg` は次の撤収で消える。手で消すなら Garage の CLI で
3. ドキュメント
   - README の「使い方」を `Database` 前提に書き直す（`grant.sh` の記述を消す）。ロードマップの 5 を完了に
   - 段階5の検証結果を README の「検証済み」に
   - ADR を足す（unstructured で扱う理由、DB ごとのバケットとキー、Secret をキャッシュしない理由）
   - `AGENTS.md` に Operator の開発手順（`make manifests generate`、開発モードの起動方法）
4. 総合試験をもう一度: `teardown.sh --yes`（ユーザーが実行する。自動の権限チェックで止められるため）→
   `bootstrap/argocd/install.sh` → `sandbox` が手作業なしで建つか

## 学んだ落とし穴（再発させない）

- k3s は起動時の IP を覚える。DHCP 予約で IP が変わったら `sudo systemctl restart k3s`（homelab の README にも記載）
- Python のスクリプトを標準ライブラリと同じ名前（`secrets.py`）にすると、自分自身が import される。`python3 -I` で起動する
- sync-wave は前の wave の Health しか待たない。同期の失敗では止まらない
- Git は空のディレクトリを記録しない。Argo CD が見ているディレクトリの最後のファイルを消すと、パスごと消えて止まる
- 既存の DB にバックアップを後付けすると、`immediate` のバックアップがサイドカーの追加より先に走って1回失敗する（Operator 経由では起きない）
- 撤収では、Operator が生きているうちに `Database` を消す（finalizer を外す係がいなくなるため）
- `garage key create` は秘密鍵を表示する。表示されるコマンドは流さない
