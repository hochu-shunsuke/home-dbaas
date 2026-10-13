# home-dbaas

自宅のミニPC（hochu-ubuntu-1 / k3s）の上に作る、学習用の Postgres DB クラウド。
「DB が欲しい」と言えば数十秒で払い出され、消せば跡形もなく消える。そこを目指す。

## 構造

本物のクラウド（RDS や Neon）と同じく、2層に分ける。

```
コントロールプレーン（自作）   DB の払い出し・一覧・削除・ブランチ       cmd/  databases/
        │ Cluster を作る
データプレーン（CloudNativePG）  Postgres の起動・フェイルオーバー・バックアップ
```

| ディレクトリ | 中身 |
|---|---|
| `infra/` | 入れたもの（CNPG operator など）。バージョン固定 |
| `databases/` | 作ったもの。DB 1つ = ファイル1枚 |
| `bootstrap/` | Argo CD の導入（段階2） |
| `cmd/` | `hdb` CLI と API（段階4） |
| `docs/decisions/` | 設計判断の記録（ADR） |

汎用の基盤にはしていない。理由は [ADR 0001](docs/decisions/0001-service-specific.md)。

## 使い方

```bash
mise install                              # helm / kubectl
infra/cnpg-operator/install.sh diff       # 何が入るか見る
infra/cnpg-operator/install.sh            # operator を入れる
kubectl apply -f databases/sandbox.yaml   # DB を1つ作る
kubectl -n db-sandbox get cluster         # STATUS が "Cluster in healthy state" になるまで待つ
```

接続情報（パスワードは CNPG が生成する）:

```bash
kubectl -n db-sandbox get secret sandbox-app -o jsonpath='{.data.password}' | base64 -d
```

| 接続元 | 接続先 |
|---|---|
| 自宅 LAN | `postgresql://app:<pw>@192.168.0.240:30432/app?sslmode=require` |
| 外出先（tailnet） | `postgresql://app:<pw>@100.78.118.56:30432/app?sslmode=require` |

## ロードマップ

| 段階 | 内容 | 学ぶこと |
|---|---|---|
| **1** | CNPG で Postgres を1つ建て、Mac から繋ぐ | Operator と CRD |
| 2 | Argo CD を入れ、`databases/` を GitOps にする | GitOps |
| 3 | cert-manager + Barman Cloud Plugin + Garage で定期バックアップ。消して復元する訓練 | バックアップと DR |
| 4 | Go で `hdb` CLI / API（create / list / delete / connect） | API 設計、client-go |
| 5 | ブランチ（PITR で複製）、プラン（Quota の型）、テナント分離（NetworkPolicy） | マルチテナント |
| 6 | Traefik の Postgres STARTTLS + HostSNI で、ポート1つ・ホスト名で全 DB に入れる | L4 ルーティング、TLS |
| 7 | 監視（CNPG の PodMonitor + Grafana）、Valkey を足してマルチエンジン化 | 観測、拡張性 |

## 検証済み（段階1・2026-10-13）

| 確認 | 結果 |
|---|---|
| Ubuntu 内: `kubectl exec ... psql -c 'select version()'` | PostgreSQL 18.6 |
| Ubuntu → `192.168.0.240:30432` / `100.78.118.56:30432`（`nc -z`） | 両方 OK |
| Mac（外出先）→ tailnet `100.78.118.56:30432`、`sslmode=require` | 接続・TLS とも OK |
| Mac（自宅 LAN）→ `192.168.0.240:30432` | **未確認**（自宅に戻ったら試す） |
| `kubectl delete -f` → namespace・PV とも消える → `apply` で再作成 | OK |
| 使用メモリ（アイドル時） | operator 30Mi、Postgres 1台 49Mi |

`kubectl wait --for=condition=Ready` が通った直後の数秒は、NodePort が `Connection refused` になる。
primary のラベルが Pod に付くのが Ready より少し遅いため。少し待てば繋がる。
