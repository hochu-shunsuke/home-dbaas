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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Plan は DB の大きさの型。メモリとディスクの上限がまとめて決まる。
// +kubebuilder:validation:Enum=small;medium
type Plan string

const (
	PlanSmall  Plan = "small"
	PlanMedium Plan = "medium"
)

// DatabaseSpec は利用者が書く「欲しい DB」。
type DatabaseSpec struct {
	// plan は作成後に変えられない。local-path のディスクは後から広げられないため。
	// +kubebuilder:default=small
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="plan は作成後に変更できない"
	// +optional
	Plan Plan `json:"plan,omitempty"`

	// backup が true なら、専用バケットと専用キーを発行し、WAL の継続アーカイブと毎日のバックアップを行う。
	// +kubebuilder:default=true
	// +optional
	Backup *bool `json:"backup,omitempty"`
}

// DatabaseStatus は Operator が書き込む「いまの DB」。
type DatabaseStatus struct {
	// phase は CNPG の Cluster の状態をそのまま写したもの。
	// +optional
	Phase string `json:"phase,omitempty"`

	// namespace は DB の中身が置かれる namespace（db-<name>）。
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// nodePort は機体の外から繋ぐポート。k8s が空きを選ぶ。
	// +optional
	NodePort int32 `json:"nodePort,omitempty"`

	// credentialsSecret はアプリ用ユーザーの接続情報が入った Secret の名前（namespace は status.namespace）。
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`

	// bucket はこの DB 専用のバックアップ置き場。
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// observedGeneration は status が反映している spec の世代。
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions の Ready が True なら、DB に接続できる。
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=db
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.plan`
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=`.status.nodePort`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// バケット名（db-<name>-<uid 8桁>）が S3 の上限 63 文字に収まるように、名前を短く制限する。
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 40",message="名前は40文字以内"

// Database は「Postgres を1つ貸してほしい」という依頼。
// クラスタ全体のリソースで、1つにつき namespace を1つ丸ごと持つ。
type Database struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec DatabaseSpec `json:"spec"`

	// +optional
	Status DatabaseStatus `json:"status,omitzero"`
}

// BackupEnabled は spec.backup の既定値（true）を含めて解釈する。
func (d *Database) BackupEnabled() bool {
	return d.Spec.Backup == nil || *d.Spec.Backup
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Database{}, &DatabaseList{})
		return nil
	})
}
