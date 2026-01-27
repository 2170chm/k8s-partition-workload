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

	// Template describes the pods that will be created.
	// +required
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Template v1.PodTemplateSpec `json:"template"`

	// Partition describes the number of pods that are updated when a
	// revision is made to spec.Template. The remaining rest of the pods
	// (spec.Replicas - spec.Partition) will stay the same. Note that there can
	// be more than two versions of pods. When the desired state is reached,
	// only spec.Partition number of pods are guaranteed to have the latest
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

	// ReadyReplicas is the number of Pods created by the PartitionWorkload controller that have a Ready Condition.
	// +kubebuilder:validation:Minimum=0
	ReadyReplicas int32 `json:"readyReplicas"`

	// UpdatedReplicas is the number of Pods created by the PartitionWorkload controller from the PartitionWorkload version
	// indicated by updateRevision.
	// +kubebuilder:validation:Minimum=0
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// UpdateRevision, if not empty, indicates the latest revision of the PartitionWorkload.
	UpdateRevision string `json:"updateRevision,omitempty"`
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
