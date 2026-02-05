package sync

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Interface interface {
	ScaleAndUpdate(
		currentPW, updatedPW *workloadv1alpha1.PartitionWorkload,
		currentRevision, updateRevision string,
		pods []*v1.Pod,
	) error
}

type realSync struct {
	client.Client
}

func NewSync(c client.Client) Interface {
	return &realSync{
		Client: c,
	}
}
