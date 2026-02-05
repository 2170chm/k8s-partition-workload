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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PartitionWorkloadSpec defines the desired state of PartitionWorkload
type PartitionWorkloadSpec struct {

	// Replicas is the desired number of replicas of the given Template.
	// These are replicas in the sense that they are instantiations of the
	// same Template.
	// If unspecified, defaults to 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Selector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// +required
	Selector *metav1.LabelSelector `json:"selector"`

	// Template describes the pods that will be created.
	// +required
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Template v1.PodTemplateSpec `json:"template"`

	// Partition describes the number of pods that are at the latest pod template revision
	// when revision is made to spec.Template. The remaining rest of the pods
	// (spec.Replicas - spec.Partition) can be of any version (but not the latest version).
	// Note that there can be more than two versions of pods. When the desired state is reached,
	// exactly spec.Partition number of pods have the latest
	// version. It defaults to spec.Replicas by controller logic.
	// +kubebuilder:validation:Minimum=0
	Partition *int32 `json:"partition,omitempty"`
}

// PartitionWorkloadStatus defines the observed state of PartitionWorkload.
type PartitionWorkloadStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// ObservedGeneration is the most recent generation observed for this PartitionWorkload. It corresponds to the
	// PartitionWorkload's generation, which is updated on mutation by the API Server.
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReadyReplicas is the number of Pods created by the PartitionWorkload controller
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"readyReplicas"`

	// UpdatedReplicas is the number of Pods created by the PartitionWorkload controller from the PartitionWorkload version
	// indicated by updateRevision.
	// +kubebuilder:validation:Minimum=0
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// CurrentRevision, if not empty, indicates the current revision version of the PartitionWorkload.
	CurrentRevision string `json:"currentRevision,omitempty"`

	// UpdateRevision, if not empty, indicates the latest revision of the PartitionWorkload.
	UpdateRevision string `json:"updateRevision,omitempty"`

	// CollisionCount is the count of hash collisions for the PartitionWorkload. The PartitionWorkload controller
	// uses this field as a collision avoidance mechanism when it needs to create the name for the
	// newest ControllerRevision.
	CollisionCount *int32 `json:"collisionCount,omitempty"`

	// Conditions represents the latest available observations of a PartitionWorkload's current state.
	Conditions []PartitionWorkloadCondition `json:"conditions,omitempty"`
}

type PartitionWorkloadConditionType string

const (
	ConditionFailedScale PartitionWorkloadConditionType = "FailedScale"
)

type PartitionWorkloadCondition struct {
	// Type of PartitionWorkload condition.
	Type PartitionWorkloadConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// Last time the condition is updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PartitionWorkload is the Schema for the partitionworkloads API
type PartitionWorkload struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of PartitionWorkload
	// +required
	Spec PartitionWorkloadSpec `json:"spec"`

	// status defines the observed state of PartitionWorkload
	// +optional
	Status PartitionWorkloadStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PartitionWorkloadList contains a list of PartitionWorkload
type PartitionWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PartitionWorkload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PartitionWorkload{}, &PartitionWorkloadList{})
}
