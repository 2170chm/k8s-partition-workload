package status

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func UpdateStatus(instance *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus, pods []*v1.Pod) error {
	return nil
}
