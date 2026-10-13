package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	dbaasv1alpha1 "github.com/hochu-shunsuke/home-dbaas/operator/api/v1alpha1"
)

// CNPG と Barman のリソースは Go の型を import せず unstructured で組む。
// 依存ライブラリ同士の版の衝突を避けるのと、作るものが YAML と1対1で読めるようにするため。
var (
	gvkCluster        = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Cluster"}
	gvkScheduledBkp   = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "ScheduledBackup"}
	gvkObjectStore    = schema.GroupVersionKind{Group: "barmancloud.cnpg.io", Version: "v1", Kind: "ObjectStore"}
	postgresImage     = "ghcr.io/cloudnative-pg/postgresql:18.6-system-trixie"
	barmanPluginName  = "barman-cloud.cloudnative-pg.io"
	backupSecretName  = "backup-s3"
	objectStoreName   = "backup"
	externalSvcSuffix = "-external"
)

// planSpec はプランごとの上限。Quota は Postgres 本体 + バックアップのサイドカー + 余裕。
type planSpec struct {
	memoryLimit, memoryRequest, storage, quotaMemory, quotaStorage string
}

var plans = map[dbaasv1alpha1.Plan]planSpec{
	dbaasv1alpha1.PlanSmall:  {memoryLimit: "512Mi", memoryRequest: "256Mi", storage: "2Gi", quotaMemory: "1Gi", quotaStorage: "4Gi"},
	dbaasv1alpha1.PlanMedium: {memoryLimit: "1Gi", memoryRequest: "512Mi", storage: "5Gi", quotaMemory: "2Gi", quotaStorage: "10Gi"},
}

func planOf(db *dbaasv1alpha1.Database) planSpec {
	if p, ok := plans[db.Spec.Plan]; ok {
		return p
	}
	return plans[dbaasv1alpha1.PlanSmall]
}

// namespaceName は DB の中身を置く namespace。
func namespaceName(db *dbaasv1alpha1.Database) string { return "db-" + db.Name }

// bucketName は DB 専用のバケット名。同じ名前で作り直しても古いバックアップとぶつからないよう UID を混ぜる。
// これはキーの名前にも使う。
func bucketName(db *dbaasv1alpha1.Database) string {
	return fmt.Sprintf("db-%s-%s", db.Name, string(db.UID)[:8])
}

func newObject(gvk schema.GroupVersionKind, namespace, name string, spec map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

func clusterObject(db *dbaasv1alpha1.Database) *unstructured.Unstructured {
	p := planOf(db)
	spec := map[string]any{
		"instances": int64(1), // 1台構成ではレプリカも同じ機体に乗るので HA にならない
		"imageName": postgresImage,
		"storage":   map[string]any{"size": p.storage},
		"resources": map[string]any{
			"requests": map[string]any{"memory": p.memoryRequest, "cpu": "100m"},
			"limits":   map[string]any{"memory": p.memoryLimit},
		},
		"bootstrap": map[string]any{"initdb": map[string]any{"database": "app", "owner": "app"}},
	}
	if db.BackupEnabled() {
		spec["plugins"] = []any{map[string]any{
			"name":          barmanPluginName,
			"isWALArchiver": true,
			"parameters":    map[string]any{"barmanObjectName": objectStoreName},
		}}
	}
	return newObject(gvkCluster, namespaceName(db), db.Name, spec)
}

func objectStoreObject(db *dbaasv1alpha1.Database, s3URL string) *unstructured.Unstructured {
	return newObject(gvkObjectStore, namespaceName(db), objectStoreName, map[string]any{
		"retentionPolicy": "7d",
		"configuration": map[string]any{
			"destinationPath": "s3://" + bucketName(db) + "/",
			"endpointURL":     s3URL,
			"s3Credentials": map[string]any{
				"accessKeyId":     map[string]any{"name": backupSecretName, "key": "ACCESS_KEY_ID"},
				"secretAccessKey": map[string]any{"name": backupSecretName, "key": "ACCESS_SECRET_KEY"},
			},
			"wal":  map[string]any{"compression": "gzip"},
			"data": map[string]any{"compression": "gzip"},
		},
		"instanceSidecarConfiguration": map[string]any{
			"resources": map[string]any{
				"requests": map[string]any{"memory": "64Mi", "cpu": "20m"},
				"limits":   map[string]any{"memory": "256Mi"},
			},
		},
	})
}

func scheduledBackupObject(db *dbaasv1alpha1.Database) *unstructured.Unstructured {
	return newObject(gvkScheduledBkp, namespaceName(db), db.Name+"-daily", map[string]any{
		"schedule":             "0 0 18 * * *", // UTC 18:00 = JST 03:00
		"immediate":            true,
		"backupOwnerReference": "self",
		"cluster":              map[string]any{"name": db.Name},
		"method":               "plugin",
		"pluginConfiguration":  map[string]any{"name": barmanPluginName},
	})
}
