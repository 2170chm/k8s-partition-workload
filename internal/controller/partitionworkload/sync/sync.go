package sync

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

func SyncCloneSet(
	instance *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus,
	currentRevision, updateRevision *apps.ControllerRevision, revisions []*apps.ControllerRevision,
	filteredPods []*v1.Pod,
) error {
	return nil
}
