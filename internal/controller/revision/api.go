package revision

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Interface interface {
	NewRevision(instance *workloadv1alpha1.PartitionWorkload, revision int64, collisionCount *int32) (*apps.ControllerRevision, error)
	getPatch(instance *workloadv1alpha1.PartitionWorkload) ([]byte, error)
	ApplyRevision(instance *workloadv1alpha1.PartitionWorkload, revision *apps.ControllerRevision) (*workloadv1alpha1.PartitionWorkload, error)
}

type realRevision struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewRevisionControl(c client.Client, s *runtime.Scheme) Interface {
	return &realRevision{
		Client: c,
		Scheme: s,
	}
}
