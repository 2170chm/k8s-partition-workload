package status

import (
	"context"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func (r *realStatusUpdater) UpdateStatus(pw *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus, pods []*v1.Pod) error {
	r.calculateStatus(pw, newStatus, pods)
	if !r.inconsistentStatus(pw, newStatus) {
		return nil
	}
	klog.InfoS("To update PartitionWorkload status", "PartitionWorkload", klog.KObj(pw), "replicas", newStatus.Replicas, "updated replicas", newStatus.UpdatedReplicas,
		"currentRevision", newStatus.CurrentRevision, "updateRevision", newStatus.UpdateRevision)
	return r.commitStatusUpdate(pw, newStatus)
}

func (r *realStatusUpdater) commitStatusUpdate(pw *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		clone := &workloadv1alpha1.PartitionWorkload{}
		if err := r.Get(context.TODO(), types.NamespacedName{Namespace: pw.Namespace, Name: pw.Name}, clone); err != nil {
			return err
		}
		clone.Status = *newStatus
		return r.Status().Update(context.TODO(), clone)
	})
}

func (r *realStatusUpdater) calculateStatus(pw *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus, pods []*v1.Pod) {
	for _, pod := range pods {
		newStatus.Replicas++
		if generalutil.EqualToRevisionHash(pod, newStatus.UpdateRevision) {
			newStatus.UpdatedReplicas++
		}
	}
	// Consider update revision to be stable and set current revision as it if all replicas are at update revision (full rollout)
	if newStatus.UpdatedReplicas == newStatus.Replicas && newStatus.Replicas == *pw.Spec.Replicas {
		newStatus.CurrentRevision = newStatus.UpdateRevision
	}
}

func (r *realStatusUpdater) inconsistentStatus(pw *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus) bool {
	oldStatus := pw.Status
	return newStatus.ObservedGeneration > oldStatus.ObservedGeneration ||
		newStatus.Replicas != oldStatus.Replicas ||
		newStatus.UpdatedReplicas != oldStatus.UpdatedReplicas ||
		newStatus.UpdateRevision != oldStatus.UpdateRevision ||
		newStatus.CurrentRevision != oldStatus.CurrentRevision
}
