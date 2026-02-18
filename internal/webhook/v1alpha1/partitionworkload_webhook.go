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
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// nolint:unused
// log is for logging in this package.
var partitionworkloadlog = logf.Log.WithName("partitionworkload-resource")

// SetupPartitionWorkloadWebhookWithManager registers the webhook for PartitionWorkload in the manager.
func SetupPartitionWorkloadWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &workloadv1alpha1.PartitionWorkload{}).
		WithValidator(&PartitionWorkloadCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-workload-scott-dev-v1alpha1-partitionworkload,mutating=false,failurePolicy=fail,sideEffects=None,groups=workload.scott.dev,resources=partitionworkloads,verbs=create;update,versions=v1alpha1,name=vpartitionworkload-v1alpha1.kb.io,admissionReviewVersions=v1

// PartitionWorkloadCustomValidator struct is responsible for validating the PartitionWorkload resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PartitionWorkloadCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type PartitionWorkload.
func (v *PartitionWorkloadCustomValidator) ValidateCreate(_ context.Context, obj *workloadv1alpha1.PartitionWorkload) (admission.Warnings, error) {
	partitionworkloadlog.Info("Validation for PartitionWorkload upon creation", "name", obj.GetName())
	var allErrs field.ErrorList

	partition := obj.Spec.Partition
	replicas := obj.Spec.Replicas

	if partition == nil {
		return nil, nil
	}
	if replicas == nil {
		if *partition > 1 {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "partition"), *partition, "must be <= 1 when spec.replicas is nil"),
			)
		}
	} else {
		if *partition > *replicas {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "partition"), *partition, "must be <= spec.replicas"),
			)
		}
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "workload.scott.dev", Kind: "PartitionWorkload"},
			obj.GetName(),
			allErrs,
		)
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type PartitionWorkload.
func (v *PartitionWorkloadCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *workloadv1alpha1.PartitionWorkload) (admission.Warnings, error) {
	partitionworkloadlog.Info("Validation for PartitionWorkload upon creation", "name", newObj.GetName())
	var allErrs field.ErrorList

	partition := newObj.Spec.Partition
	replicas := newObj.Spec.Replicas

	if partition == nil {
		return nil, nil
	}
	if replicas == nil {
		if *partition > 1 {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "partition"), *partition, "must be <= 1 when spec.replicas is nil"),
			)
		}
	} else {
		if *partition > *replicas {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "partition"), *partition, "must be <= spec.replicas"),
			)
		}
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "workload.scott.dev", Kind: "PartitionWorkload"},
			newObj.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type PartitionWorkload.
func (v *PartitionWorkloadCustomValidator) ValidateDelete(_ context.Context, obj *workloadv1alpha1.PartitionWorkload) (admission.Warnings, error) {
	partitionworkloadlog.Info("Validation for PartitionWorkload upon deletion", "name", obj.GetName())
	return nil, nil
}
