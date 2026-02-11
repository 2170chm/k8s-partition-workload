package status

import (
	"testing"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	sync "github.com/2170chm/k8s-partition-workload/internal/controller/sync"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
	v1 "k8s.io/api/core/v1"
)

const (
	currentRevision = "1"
	newRevision     = "2"
)

func TestCalculateStatus(t *testing.T) {
	tests := []struct {
		name           string
		pw             *workloadv1alpha1.PartitionWorkload
		pods           []*v1.Pod
		expectedStatus workloadv1alpha1.PartitionWorkloadStatus
	}{
		{
			name: "All pods are updated",
			pw:   getPW(3),
			pods: sync.NewVersionedPods(getPW(3), newRevision, 3),
			expectedStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Replicas:        3,
				UpdatedReplicas: 3,
				CurrentRevision: newRevision,
				UpdateRevision:  newRevision,
			},
		},
		{
			name: "No pods are updated",
			pw:   getPW(3),
			pods: sync.NewVersionedPods(getPW(3), currentRevision, 3),
			expectedStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Replicas:        3,
				UpdatedReplicas: 0,
				CurrentRevision: currentRevision,
				UpdateRevision:  newRevision,
			},
		},
		{
			name: "Some pods are updated",
			pw:   getPW(3),
			pods: flatten(
				sync.NewVersionedPods(getPW(3), currentRevision, 1),
				sync.NewVersionedPods(getPW(3), currentRevision, 1),
				sync.NewVersionedPods(getPW(3), newRevision, 1),
			),
			expectedStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Replicas:        3,
				UpdatedReplicas: 1,
				CurrentRevision: currentRevision,
				UpdateRevision:  newRevision,
			},
		},
		{
			name: "Less pods than expected",
			pw:   getPW(3),
			pods: flatten(
				sync.NewVersionedPods(getPW(3), currentRevision, 1),
				sync.NewVersionedPods(getPW(3), newRevision, 1),
			),
			expectedStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Replicas:        2,
				UpdatedReplicas: 1,
				CurrentRevision: currentRevision,
				UpdateRevision:  newRevision,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &realStatusUpdater{}
			newStatus := &workloadv1alpha1.PartitionWorkloadStatus{
				CurrentRevision: currentRevision,
				UpdateRevision:  newRevision,
			}

			updater.calculateStatus(tt.pw, newStatus, tt.pods)

			if newStatus.Replicas != tt.expectedStatus.Replicas {
				t.Errorf("Replicas = %d, want %d", newStatus.Replicas, tt.expectedStatus.Replicas)
			}
			if newStatus.UpdatedReplicas != tt.expectedStatus.UpdatedReplicas {
				t.Errorf("Updated Replicas = %d, want %d", newStatus.UpdatedReplicas, tt.expectedStatus.UpdatedReplicas)
			}
			if newStatus.CurrentRevision != tt.expectedStatus.CurrentRevision {
				t.Errorf("CurrentRevision = %s, want %s", newStatus.CurrentRevision, tt.expectedStatus.CurrentRevision)
			}
			if newStatus.UpdateRevision != tt.expectedStatus.UpdateRevision {
				t.Errorf("UpdateRevision = %s, want %s", newStatus.UpdateRevision, tt.expectedStatus.UpdateRevision)
			}
		})
	}
}

func getPW(replicas int32) *workloadv1alpha1.PartitionWorkload {
	return &workloadv1alpha1.PartitionWorkload{
		Spec: workloadv1alpha1.PartitionWorkloadSpec{
			Replicas: generalutil.Int32Ptr(replicas),
		},
	}
}

func flatten(podSlices ...[]*v1.Pod) []*v1.Pod {
	var pods []*v1.Pod
	for _, slice := range podSlices {
		pods = append(pods, slice...)
	}
	return pods
}
