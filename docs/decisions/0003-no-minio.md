# 0003 S3 互換ストレージに MinIO を使わない

日付: 2026-10-13

## 決定

バックアップ置き場には Garage を使う（`singleNode: true` で replication_factor = 1）。

## 理由

- MinIO のオープンソース版は 2026-04 にリポジトリがアーカイブされ、それ以前にコンソール機能の削除やイメージ配布の停止があった
- Garage は小規模・少ノード向けに作られており、単一ノードから始めて後でノードを足せる
- 他候補: RustFS（MinIO に最も近いが新しい）、SeaweedFS（大量ファイル向け）、Ceph RGW（重すぎる）

## 未確認

- Barman Cloud Plugin から Garage へ繋ぐときの設定（path-style、region 名）。段階3で実機確認して追記する
