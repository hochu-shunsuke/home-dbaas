# databases

DB 1つ = `Database` 1枚。ここに置いて push すると、自作の Operator が DB を用意する。消して push すると、
バックアップのバケットと鍵まで含めて片付ける（データも消える）。

```yaml
apiVersion: dbaas.hochu.dev/v1alpha1
kind: Database
metadata:
  name: foo          # 40 文字以内。namespace は db-foo になる
spec:
  plan: small        # small（512Mi / 2Gi）か medium（1Gi / 5Gi）。作成後は変えられない
  # backup: false    # 既定は true（専用バケットへ WAL の継続アーカイブ + 毎日のバックアップ）
```

接続先は `kubectl get database` の PORT 列。パスワードは `db-foo` の Secret `foo-app`。

このファイルは消さない。中身が空になると Git はディレクトリごと記録しなくなり、
Argo CD が「パスが無い」で止まる（安全側に倒れて何も消さないが、何も反映もされない）。
