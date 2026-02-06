package status

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Interface interface {
	UpdateStatus(pw *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus, pods []*v1.Pod) error
}

type realStatusUpdater struct {
	client.Client
}

func NewStatusUpdater(c client.Client) Interface {
	return &realStatusUpdater{
		Client: c,
	}
}
