package pods

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func GetOwnedPods(instance *workloadv1alpha1.PartitionWorkload) ([]*v1.Pod, error) {
	panic("unimplemented")
}

func ClaimPods(instance *workloadv1alpha1.PartitionWorkload, pods []*v1.Pod) ([]*v1.Pod, error) {
	panic("unimplemented")
}
