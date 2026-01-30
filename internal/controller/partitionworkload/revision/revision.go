package revision

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

func GetActiveRevisions(instance *workloadv1alpha1.PartitionWorkload, revisions []*apps.ControllerRevision) (
	*apps.ControllerRevision, *apps.ControllerRevision, int32, error,
) {
	return nil, nil, 0, nil
}

func TruncateHistory(
	instance *workloadv1alpha1.PartitionWorkload,
	pods []*v1.Pod,
	revisions []*apps.ControllerRevision,
	current *apps.ControllerRevision,
	update *apps.ControllerRevision,
) error {
	return nil
}
