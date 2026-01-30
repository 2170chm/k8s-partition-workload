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

package partitionworkload

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	pods "github.com/2170chm/k8s-partition-workload/internal/controller/partitionworkload/pods"
)

// PartitionWorkloadReconciler reconciles a PartitionWorkload object
type PartitionWorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=workload.scott.dev,resources=partitionworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workload.scott.dev,resources=partitionworkloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workload.scott.dev,resources=partitionworkloads/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PartitionWorkload object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.0/pkg/reconcile
func (r *PartitionWorkloadReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	startTime := time.Now()
	defer func() {
		klog.InfoS("Finished syncing partitionworkload", "partitionworkload", request, "duration", time.Since(startTime))
	}()

	// Fetch the resource instance
	instance := &workloadv1alpha1.PartitionWorkload{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.InfoS("PartitionWorkload has been deleted", "partitionworkload", request)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.Selector)
	if err != nil {
		klog.ErrorS(err, "Error converting PartitionWorkload selector", "partitionworkload", request)
		// This is a non-transient error, so don't retry.
		return reconcile.Result{}, nil
	}

	// List active Pods owned by this PartitionWorkload
	directOwnedPods, err := pods.GetOwnedPods(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Claim/release pod ownership using label selector matching
	// This adopts pods that match our selector but aren't owned, and releases pods that don't match
	allOwnedPods, err = pods.ClaimPods(instance, directOwnedPods)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PartitionWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workloadv1alpha1.PartitionWorkload{}).
		Named("partitionworkload").
		Complete(r)
}
