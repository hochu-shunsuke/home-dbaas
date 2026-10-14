# home-dbaas

自宅のミニPC（hochu-ubuntu-1 / k3s）の上に作る、学習用の Postgres DB クラウド。
「DB が欲しい」と言えば数十秒で払い出され、消せば跡形もなく消える。そこを目指す。

## 構造

本物のクラウド（RDS や Neon）と同じく、2層に分ける。

```
コントロールプレーン（自作）   DB の払い出し・一覧・削除・ブランチ       cmd/（段階5） databases/
        │ Cluster を作る
データプレーン（CloudNativePG）  Postgres の起動・フェイルオーバー・バックアップ
```

| ディレクトリ | 中身 |
|---|---|
| `bootstrap/` | 手で流すのはここだけ。Argo CD と、`apps/` を束ねる root |
| `apps/` | Argo CD の Application。部品1つ = ファイル1枚（app-of-apps） |
| `infra/` | 入れたものの設定（Helm の values、Garage の素のマニフェスト）。バージョン固定 |
| `databases/` | 作ったもの。DB 1つ = ファイル1枚 |
| `cmd/` | Operator（段階5） |
| `docs/decisions/` | 設計判断の記録（ADR） |

**作業の現在地と再開手順は [docs/progress.md](docs/progress.md)。**

汎用の基盤にはしていない。理由は [ADR 0001](docs/decisions/0001-service-specific.md)。
これは常設の環境ではなく、学習のための一回きりの構築。影響は hochu-ubuntu-1 の中に閉じる
（クラウドのサービスは使わない。GitHub はリポジトリ置き場としてだけ使う）。

## 使い方

まっさらな k3s に建てる（手で流すのはこの1本だけ）:

```bash
mise install                    # helm / kubectl
bootstrap/argocd/install.sh     # `diff` を付けると中身を見るだけ
```

Argo CD が `apps/` を次の順に建てる。前の wave が Healthy になるまで次へ進まない。

| wave | 部品 |
|---|---|
| 0 | cert-manager、CNPG operator、Garage（秘密の生成 → 本体 → レイアウト・バケット・キー）、Argo CD の Ingress |
| 1 | Barman Cloud Plugin（cert-manager と operator が前提） |
| 2 | `databases/` |

秘密はクラスタの中で生成され、Git にもログにも出ない。
ただし DB の namespace へバックアップ用の鍵を渡す所だけは、まだ手作業（段階5で Operator に置き換える）:

```bash
infra/garage/grant.sh db-sandbox   # namespace ができてから流す
```

DB を作る・消すのは **Git だけ**。`kubectl apply` は使わない（Argo CD が Git の状態に戻してしまう）。

```bash
cp databases/sandbox.yaml databases/foo.yaml   # 中の sandbox を foo に置換、nodePort を空きに変える
git add databases/foo.yaml && git commit -m "db: foo を作る" && git push
# → 数分以内に db-foo ができる。ファイルを消して push すると DB ごと消える（データも）
```

Argo CD の画面: `http://192.168.0.240/argocd`（外出先は `http://100.78.118.56/argocd`）。
ユーザー `admin`、パスワードは
`kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d`。

接続情報（パスワードは CNPG が生成する）:

```bash
kubectl -n db-sandbox get secret sandbox-app -o jsonpath='{.data.password}' | base64 -d
```

| 接続元 | 接続先 |
|---|---|
| 自宅 LAN | `postgresql://app:<pw>@192.168.0.240:30432/app?sslmode=require` |
| 外出先（tailnet） | `postgresql://app:<pw>@100.78.118.56:30432/app?sslmode=require` |

## 撤収

k3s の中（DB のデータ・バックアップも消える。k3s 本体は残る）:

```bash
./teardown.sh          # 消す対象の一覧だけ
./teardown.sh --yes    # 実際に消す
```

k3s の外に足したもの:

| もの | 消し方 |
|---|---|
| このリポジトリ | `rm -rf ~/src/home-dbaas` |
| mise のツール（helm・kubectl） | `mise uninstall helm kubectl`（このディレクトリの `mise.toml` で入れたもの） |
| gh のログイン | `gh auth logout` |
| git の認証ヘルパー設定 | `git config --global --unset-all credential.https://github.com.helper` と `credential.https://gist.github.com.helper` |
| GitHub のリポジトリ | `gh repo delete hochu-shunsuke/home-dbaas` |

## ロードマップ

| 段階 | 内容 | 学ぶこと |
|---|---|---|
| ~~1~~ | CNPG で Postgres を1つ建て、Mac から繋ぐ | Operator と CRD |
| ~~2~~ | Argo CD を入れ、`databases/` を GitOps にする | GitOps |
| ~~3~~ | cert-manager + Barman Cloud Plugin + Garage で定期バックアップ。PITR の復元訓練 | バックアップと PITR |
| ~~4~~ | 部品も Argo CD で入れる（app-of-apps）。秘密をクラスタ内で生成。撤収スクリプト。撤収 → 再構築の総合試験 | 「1本で建つ・1本で消える」 |
| 5 | Operator: `Database` CRD。DB ごとの Garage 鍵（テナント分離）、プラン、ポートの自動割り当て、finalizer | k8s の中身 |
| 6 | Operator の機能としてブランチ（`spec.from`）と、Traefik の HostSNI でポート1つの入口 | クラウドらしさ |
| 7（任意） | 観測（Prometheus + Grafana、Operator 自身のメトリクス） | 監視 |

機体の外へのバックアップはしない（一回きりの構築で、影響を機体の中に閉じるため）。
Garage のバックアップは機体の保護ではなく、PITR・ブランチを提供する製品の機能として持つ。

## 検証済み

### 段階1（2026-10-13）

| 確認 | 結果 |
|---|---|
| Ubuntu 内: `kubectl exec ... psql -c 'select version()'` | PostgreSQL 18.6 |
| Ubuntu → `192.168.0.240:30432` / `100.78.118.56:30432`（`nc -z`） | 両方 OK |
| Mac（外出先）→ tailnet `100.78.118.56:30432`、`sslmode=require` | 接続・TLS とも OK |
| Mac（自宅 LAN）→ `192.168.0.240:30432` | 段階4の総合試験で確認済み |
| `kubectl delete -f` → namespace・PV とも消える → `apply` で再作成 | OK |
| 使用メモリ（アイドル時） | operator 30Mi、Postgres 1台 49Mi |

### 段階2（2026-10-13）

| 確認 | 結果 |
|---|---|
| 手で作った `sandbox` を Argo CD が引き取る | Synced / Healthy。Pod は作り直されない |
| `databases/demo.yaml` を push → DB ができる | push から約3分20秒で healthy（Argo CD の既定ポーリング 120s + 揺らぎ） |
| `kubectl delete svc demo-external`（手で壊す） | 3秒で Argo CD が戻した（selfHeal） |
| `demo.yaml` を消して push | 約3分で namespace・PV とも消えた（prune）。`sandbox` は無傷 |
| Argo CD の画面 `/argocd`（LAN の IP・tailnet の IP・Mac から tailnet） | すべて 200 |
| Argo CD の使用メモリ（アイドル時） | 合計 272Mi（うち application-controller 190Mi） |

### 段階3（2026-10-13）

| 確認 | 結果 |
|---|---|
| WAL の継続アーカイブ（sandbox → Garage） | `Continuous archiving is working`。path-style・region の追加設定は不要だった |
| 手動バックアップ | 6秒で完了、4.1MiB |
| **PITR 復元訓練**: `before` → T=06:11:37Z → `after` の順に挿入し、T へ戻した別 DB を作る | 復元先には `before` だけ。`after` は無い。Argo CD が拾ってから約1分20秒 |
| 使用メモリ（アイドル時） | Garage 5Mi、cert-manager 計70Mi、plugin 19Mi、サイドカー 17Mi。機体全体 35% |

既存の DB にバックアップを後付けすると、`ScheduledBackup` の `immediate` が
サイドカーの追加（Pod の再起動）より先に走り、`requested plugin is not available` で1回失敗する。
再起動後に手で `Backup` を1つ作れば通る。新規の DB では起きない見込み。

### 段階4（2026-10-13）総合試験: 撤収 → 再構築

| 確認 | 結果 |
|---|---|
| `teardown.sh --yes` | 残りは k3s 元々の namespace 4つと kube-system の Pod 7つだけ。PV なし |
| `bootstrap/argocd/install.sh` 1本からの再構築 | Argo CD の導入込みで、Ingress +29s、CNPG +34s、plugin +55s、`sandbox` の Cluster +57s |
| Garage | 初回は失敗（下記）。修正を push してから約2分で建った。秘密生成 4s、初期化 5s |
| Garage の初期化をもう一度流す | 「既にある」で何も変えずに終わる。Pod も再起動しない |
| `grant.sh db-sandbox` のあと | WAL アーカイブ再開、手動バックアップ成功 |
| Mac（自宅 LAN）→ `192.168.0.240:30432`、`sslmode=require` | 接続・TLS とも OK（段階1の宿題を解消） |
| Mac（自宅 LAN）→ Argo CD の画面 | 200 |
| 使用メモリ（全部入り・アイドル時） | 機体全体 37%（約 4.7GiB） |

わかったこと:

- **Garage の初回失敗**: 秘密生成スクリプトを `secrets.py` と名付けたため、`import secrets` が標準ライブラリでなく自分自身を読んで落ちていた。`gen_secrets.py` に改名し、`python3 -I` で起動するよう直した
- **sync-wave は Health しか見ない**: Garage の同期が失敗していても、Garage の Application の Health は Healthy だったため、wave 2 の `databases` は先に進んだ。前の wave の「同期成功」までは待たない
- **撤収の待ち時間**: DB の PVC は Postgres の Pod が完全に止まるまで消えない（pvc-protection）。180 秒では足りずスクリプトが途中で止まったので 600 秒に延ばした
- 再構築直後の自動バックアップは、`grant.sh` で鍵を渡すまで失敗する（段階5の Operator で解消する手作業）

### 補足

`kubectl wait --for=condition=Ready` が通った直後の数秒は、NodePort が `Connection refused` になる。
primary のラベルが Pod に付くのが Ready より少し遅いため。少し待てば繋がる。
