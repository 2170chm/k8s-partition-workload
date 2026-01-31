package pods

import (
	"context"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	fieldindex "github.com/2170chm/k8s-partition-workload/internal/controller/fieldindex"
	v1 "k8s.io/api/core/v1"
	fields "k8s.io/apimachinery/pkg/fields"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

func GetOwnedPods(reader client.Reader, instance *workloadv1alpha1.PartitionWorkload) ([]*v1.Pod, error) {
	opts := &client.ListOptions{
		Namespace:     instance.Namespace,
		FieldSelector: fields.SelectorFromSet(fields.Set{fieldindex.IndexNameForOwnerRefUID: string(instance.UID)}),
	}
	podList := &v1.PodList{}
	if err := reader.List(context.TODO(), podList, opts); err != nil {
		return nil, err
	}
	var activePods []*v1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if kubecontroller.IsPodActive(pod) {
			activePods = append(activePods, pod)
		}
	}
	return activePods, nil
}

func ClaimPods(instance *workloadv1alpha1.PartitionWorkload, pods []*v1.Pod) ([]*v1.Pod, error) {
	panic("unimplemented")
}
