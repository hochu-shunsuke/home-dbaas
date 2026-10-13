/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/hochu-shunsuke/home-dbaas/operator/api/v1alpha1"
	"github.com/hochu-shunsuke/home-dbaas/operator/internal/garage"
)

const (
	finalizerName = "dbaas.hochu.dev/cleanup"
	fieldOwner    = client.FieldOwner("home-dbaas-operator")
)

// DatabaseReconciler は Database を見張り、DB 1つ分の部品を揃え続ける。
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// GarageSecret は管理 API のトークンが入った Secret（namespace/name）。
	GarageSecret types.NamespacedName
	GarageAdmin  string
	GarageS3     string
}

// +kubebuilder:rbac:groups=dbaas.hochu.dev,resources=databases,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=dbaas.hochu.dev,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dbaas.hochu.dev,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces;services;resourcequotas;limitranges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;patch
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters;scheduledbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=barmancloud.cnpg.io,resources=objectstores,verbs=get;list;watch;create;update;patch;delete

// Reconcile は「Database に書かれた姿」と「クラスタの実際」を突き合わせ、足りないものを作る。
// 何度呼ばれても同じ結果になるように、作成・更新はすべて server-side apply で行う。
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	db := &dbaasv1alpha1.Database{}
	if err := r.Get(ctx, req.NamespacedName, db); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !db.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, db)
	}
	if !controllerutil.ContainsFinalizer(db, finalizerName) {
		orig := db.DeepCopy()
		controllerutil.AddFinalizer(db, finalizerName)
		if err := r.Patch(ctx, db, client.MergeFrom(orig)); err != nil {
			return ctrl.Result{}, err
		}
	}

	ns := namespaceName(db)
	if err := r.applyNamespaceAndLimits(ctx, db); err != nil {
		return r.fail(ctx, db, "Namespace", err)
	}

	if db.BackupEnabled() {
		gc, err := r.garageClient(ctx)
		if err != nil {
			return r.fail(ctx, db, "Garage", err)
		}
		if err := r.applyBackupCredentials(ctx, db, gc); err != nil {
			return r.fail(ctx, db, "Garage", err)
		}
		if err := r.applyOwned(ctx, db, objectStoreObject(db, r.GarageS3)); err != nil {
			return r.fail(ctx, db, "ObjectStore", err)
		}
	}

	cluster := clusterObject(db)
	if err := r.applyOwned(ctx, db, cluster); err != nil {
		return r.fail(ctx, db, "Cluster", err)
	}
	if db.BackupEnabled() {
		if err := r.applyOwned(ctx, db, scheduledBackupObject(db)); err != nil {
			return r.fail(ctx, db, "ScheduledBackup", err)
		}
	}
	if err := r.applyExternalService(ctx, db); err != nil {
		return r.fail(ctx, db, "Service", err)
	}

	// ここからは status の更新。CNPG の Cluster の状態と、k8s が選んだ NodePort を写す
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
		return ctrl.Result{}, err
	}
	phase, _, _ := unstructured.NestedString(cluster.Object, "status", "phase")
	ready, _, _ := unstructured.NestedInt64(cluster.Object, "status", "readyInstances")

	svc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: db.Name + externalSvcSuffix}, svc); err != nil {
		return ctrl.Result{}, err
	}

	orig := db.DeepCopy()
	db.Status.Phase = phase
	db.Status.Namespace = ns
	db.Status.CredentialsSecret = db.Name + "-app"
	db.Status.ObservedGeneration = db.Generation
	if len(svc.Spec.Ports) > 0 {
		db.Status.NodePort = svc.Spec.Ports[0].NodePort
	}
	if db.BackupEnabled() {
		db.Status.Bucket = bucketName(db)
	}
	cond := metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Provisioning", Message: phase,
		ObservedGeneration: db.Generation}
	if ready >= 1 && phase == "Cluster in healthy state" {
		cond.Status, cond.Reason = metav1.ConditionTrue, "ClusterHealthy"
	}
	meta.SetStatusCondition(&db.Status.Conditions, cond)
	// Update（丸ごと上書き）だと、読んだ後に誰かが書き換えていたら競合で失敗する。変えた所だけ送る
	if err := r.Status().Patch(ctx, db, client.MergeFrom(orig)); err != nil {
		return ctrl.Result{}, err
	}

	log.V(1).Info("reconciled", "phase", phase, "nodePort", db.Status.NodePort)
	return ctrl.Result{}, nil
}

// fail は失敗を status に書いてから、controller-runtime に再試行を任せる。
func (r *DatabaseReconciler) fail(ctx context.Context, db *dbaasv1alpha1.Database, step string, err error) (ctrl.Result, error) {
	orig := db.DeepCopy()
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: step + "Failed", Message: err.Error(),
		ObservedGeneration: db.Generation,
	})
	if uerr := r.Status().Patch(ctx, db, client.MergeFrom(orig)); uerr != nil {
		logf.FromContext(ctx).Error(uerr, "status を更新できない")
	}
	return ctrl.Result{}, fmt.Errorf("%s: %w", step, err)
}

func (r *DatabaseReconciler) applyNamespaceAndLimits(ctx context.Context, db *dbaasv1alpha1.Database) error {
	ns := namespaceName(db)
	p := planOf(db)

	nsObj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: ns, Labels: map[string]string{
			"app.kubernetes.io/part-of": "home-dbaas",
			"dbaas.hochu.dev/database":  db.Name,
		}},
	}
	quota := &corev1.ResourceQuota{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ResourceQuota"},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "quota"},
		Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{
			corev1.ResourceRequestsMemory:         resource.MustParse(p.quotaMemory),
			corev1.ResourceLimitsMemory:           resource.MustParse(p.quotaMemory),
			corev1.ResourceRequestsStorage:        resource.MustParse(p.quotaStorage),
			corev1.ResourcePersistentVolumeClaims: resource.MustParse("2"),
		}},
	}
	// Quota で limits.memory を縛ると、limits の無い Pod は作れない。CNPG が裏で作る Job 向けの既定値
	limits := &corev1.LimitRange{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "LimitRange"},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "defaults"},
		Spec: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{{
			Type:           corev1.LimitTypeContainer,
			DefaultRequest: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi"), corev1.ResourceCPU: resource.MustParse("50m")},
			Default:        corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
		}}},
	}
	for _, obj := range []client.Object{nsObj, quota, limits} {
		if err := r.applyOwned(ctx, db, obj); err != nil {
			return err
		}
	}
	return nil
}

// applyBackupCredentials は DB 専用のバケットとキーを Garage に用意し、キーを Secret に置く。
// キーは自分のバケットにしか権限が無いので、DB 同士で互いのバックアップは読めない。
func (r *DatabaseReconciler) applyBackupCredentials(ctx context.Context, db *dbaasv1alpha1.Database, gc *garage.Client) error {
	name := bucketName(db)
	bucket, err := gc.EnsureBucket(ctx, name)
	if err != nil {
		return err
	}
	key, err := gc.EnsureKey(ctx, name)
	if err != nil {
		return err
	}
	if err := gc.Allow(ctx, bucket.ID, key.AccessKeyID); err != nil {
		return err
	}
	return r.applyOwned(ctx, db, &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName(db), Name: backupSecretName},
		Data: map[string][]byte{"ACCESS_KEY_ID": []byte(key.AccessKeyID), "ACCESS_SECRET_KEY": []byte(key.SecretAccessKey)},
	})
}

// applyExternalService は機体の外から繋ぐ入口。nodePort は書かず、k8s に空きを選ばせる。
func (r *DatabaseReconciler) applyExternalService(ctx context.Context, db *dbaasv1alpha1.Database) error {
	return r.applyOwned(ctx, db, &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName(db), Name: db.Name + externalSvcSuffix},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"cnpg.io/cluster": db.Name, "cnpg.io/instanceRole": "primary"},
			Ports: []corev1.ServicePort{{
				Name: "postgres", Port: 5432, TargetPort: intstr.FromInt32(5432), Protocol: corev1.ProtocolTCP,
			}},
		},
	})
}

// applyOwned は obj に「親は db」という印（ownerReference）を付けて server-side apply する。
// Database を消すと、印を辿って k8s のガベージコレクタが子を消す。
func (r *DatabaseReconciler) applyOwned(ctx context.Context, db *dbaasv1alpha1.Database, obj client.Object) error {
	if err := controllerutil.SetControllerReference(db, obj, r.Scheme); err != nil {
		return err
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return err
		}
		u = &unstructured.Unstructured{Object: raw}
		// 空の status や creationTimestamp を送ると、それも「自分の持ち物」と宣言してしまう
		delete(u.Object, "status")
		unstructured.RemoveNestedField(u.Object, "metadata", "creationTimestamp")
	}
	return r.Apply(ctx, client.ApplyConfigurationFromUnstructured(u), fieldOwner, client.ForceOwnership)
}

// finalize は Database が消されるときの後片付け。
// 先に namespace（中の Postgres）を消し、WAL の書き込みが止まってからバケットとキーを消す。
func (r *DatabaseReconciler) finalize(ctx context.Context, db *dbaasv1alpha1.Database) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(db, finalizerName) {
		return ctrl.Result{}, nil
	}
	log := logf.FromContext(ctx)

	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: namespaceName(db)}, ns)
	switch {
	case err == nil:
		if ns.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, ns); client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}
		}
		log.Info("namespace の削除を待つ", "namespace", ns.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	case !apierrors.IsNotFound(err):
		return ctrl.Result{}, err
	}

	if db.BackupEnabled() {
		gc, err := r.garageClient(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := gc.DeleteBucketAndKey(ctx, bucketName(db)); err != nil {
			return ctrl.Result{}, err
		}
	}

	orig := db.DeepCopy()
	controllerutil.RemoveFinalizer(db, finalizerName)
	return ctrl.Result{}, r.Patch(ctx, db, client.MergeFrom(orig))
}

func (r *DatabaseReconciler) garageClient(ctx context.Context) (*garage.Client, error) {
	s := &corev1.Secret{}
	if err := r.Get(ctx, r.GarageSecret, s); err != nil {
		return nil, fmt.Errorf("Garage の管理トークン %s を読めない: %w", r.GarageSecret, err)
	}
	return &garage.Client{AdminURL: r.GarageAdmin, S3URL: r.GarageS3, Token: string(s.Data["admin_token"])}, nil
}

// SetupWithManager は「何が変わったら Reconcile を呼ぶか」を登録する。
// 自分が作った子（Cluster・Service・Namespace）が変わったら、親の Database を呼び直す。
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(gvkCluster)
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.Database{}).
		Owns(cluster).
		Owns(&corev1.Service{}).
		Owns(&corev1.Namespace{}).
		Named("database").
		Complete(r)
}
