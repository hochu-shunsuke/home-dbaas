# cmd

段階4で Go のコントロールプレーンを置く。

- `cmd/hdb/` — CLI（`hdb create / list / delete / connect`）
- `cmd/api/` — API。DB の作成依頼を受けて `databases/` 相当のリソースを作る
