package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	dbaasv1alpha1 "github.com/hochu-shunsuke/home-dbaas/operator/api/v1alpha1"
)

func newDB(name string, plan dbaasv1alpha1.Plan, backup *bool) *dbaasv1alpha1.Database {
	db := &dbaasv1alpha1.Database{Spec: dbaasv1alpha1.DatabaseSpec{Plan: plan, Backup: backup}}
	db.Name = name
	db.UID = types.UID("1234abcd-0000-0000-0000-000000000000")
	return db
}

func TestNames(t *testing.T) {
	db := newDB("foo", dbaasv1alpha1.PlanSmall, nil)
	if got := namespaceName(db); got != "db-foo" {
		t.Errorf("namespace = %q", got)
	}
	// 同じ名前で作り直しても古いバケットとぶつからないよう UID の先頭8桁が付く
	if got := bucketName(db); got != "db-foo-1234abcd" {
		t.Errorf("bucket = %q", got)
	}
	// 名前の上限 40 文字でも S3 のバケット名の上限 63 文字に収まる
	long := newDB("a234567890123456789012345678901234567890", dbaasv1alpha1.PlanSmall, nil)
	if n := len(bucketName(long)); n > 63 {
		t.Errorf("bucket name too long: %d", n)
	}
}

func TestClusterFollowsPlanAndBackup(t *testing.T) {
	f := false
	for _, tc := range []struct {
		db         *dbaasv1alpha1.Database
		mem, disk  string
		wantPlugin bool
	}{
		{newDB("a", dbaasv1alpha1.PlanSmall, nil), "512Mi", "2Gi", true},
		{newDB("b", dbaasv1alpha1.PlanMedium, nil), "1Gi", "5Gi", true},
		{newDB("c", "", &f), "512Mi", "2Gi", false}, // plan 未指定は small 扱い
	} {
		c := clusterObject(tc.db)
		mem, _, _ := unstructured.NestedString(c.Object, "spec", "resources", "limits", "memory")
		disk, _, _ := unstructured.NestedString(c.Object, "spec", "storage", "size")
		_, hasPlugin, _ := unstructured.NestedSlice(c.Object, "spec", "plugins")
		if mem != tc.mem || disk != tc.disk || hasPlugin != tc.wantPlugin {
			t.Errorf("%s: mem=%s disk=%s plugin=%v", tc.db.Name, mem, disk, hasPlugin)
		}
		if c.GetNamespace() != "db-"+tc.db.Name || c.GetName() != tc.db.Name {
			t.Errorf("%s: placed at %s/%s", tc.db.Name, c.GetNamespace(), c.GetName())
		}
	}
}

func TestObjectStorePointsToOwnBucket(t *testing.T) {
	o := objectStoreObject(newDB("foo", dbaasv1alpha1.PlanSmall, nil), "http://garage:3900")
	dest, _, _ := unstructured.NestedString(o.Object, "spec", "configuration", "destinationPath")
	if dest != "s3://db-foo-1234abcd/" {
		t.Errorf("destinationPath = %q", dest)
	}
}
