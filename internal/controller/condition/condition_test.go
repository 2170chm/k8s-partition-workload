package condition

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
)

func TestGetPartitionWorkloadCondition(t *testing.T) {
	condType := workloadv1alpha1.PartionWorkloadConditionFailedScale
	condition := workloadv1alpha1.PartitionWorkloadCondition{
		Type:   condType,
		Status: v1.ConditionTrue,
	}

	tests := []struct {
		name       string
		status     workloadv1alpha1.PartitionWorkloadStatus
		condType   workloadv1alpha1.PartitionWorkloadConditionType
		wantExist  bool
		wantStatus v1.ConditionStatus
	}{
		{
			name: "Condition exists",
			status: workloadv1alpha1.PartitionWorkloadStatus{
				Conditions: []workloadv1alpha1.PartitionWorkloadCondition{condition},
			},
			condType:   condType,
			wantExist:  true,
			wantStatus: v1.ConditionTrue,
		},
		{
			name: "Condition not exists",
			status: workloadv1alpha1.PartitionWorkloadStatus{
				Conditions: []workloadv1alpha1.PartitionWorkloadCondition{},
			},
			condType:  condType,
			wantExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCondition(tt.status, tt.condType)
			if tt.wantExist && got == nil {
				t.Errorf("GetCondition() = nil, want non-nil")
			}
			if !tt.wantExist && got != nil {
				t.Errorf("GetCondition() = %v, want nil", got)
			}
			if got != nil && got.Status != tt.wantStatus {
				t.Errorf("GetCondition().Status = %v, want %v", got.Status, tt.wantStatus)
			}
		})
	}
}

func getNewCondition(condType workloadv1alpha1.PartitionWorkloadConditionType, status v1.ConditionStatus,
	now time.Time) *workloadv1alpha1.PartitionWorkloadCondition {
	return &workloadv1alpha1.PartitionWorkloadCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.NewTime(now),
		LastTransitionTime: metav1.NewTime(now),
	}
}

func TestSetPartitionWorkloadCondition(t *testing.T) {
	now := time.Now()
	condType := workloadv1alpha1.PartionWorkloadConditionFailedScale

	tests := []struct {
		name           string
		initialStatus  workloadv1alpha1.PartitionWorkloadStatus
		newCondition   *workloadv1alpha1.PartitionWorkloadCondition
		expectedStatus workloadv1alpha1.PartitionWorkloadStatus
	}{
		{
			name: "Update existing condition with different status",
			initialStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Conditions: []workloadv1alpha1.PartitionWorkloadCondition{
					{
						Type:               condType,
						Status:             v1.ConditionTrue,
						LastUpdateTime:     metav1.NewTime(now.Add(-time.Minute)),
						LastTransitionTime: metav1.NewTime(now.Add(-time.Minute)),
					},
				},
			},
			newCondition: getNewCondition(condType, v1.ConditionFalse, now.Add(time.Second)),
			expectedStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Conditions: []workloadv1alpha1.PartitionWorkloadCondition{
					{
						Type:               condType,
						Status:             v1.ConditionFalse,
						LastUpdateTime:     metav1.NewTime(now.Add(time.Second)),
						LastTransitionTime: metav1.NewTime(now.Add(time.Second)),
					},
				},
			},
		},
		{
			name: "Update existing condition with same condition",
			initialStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Conditions: []workloadv1alpha1.PartitionWorkloadCondition{
					{
						Type:               condType,
						Status:             v1.ConditionTrue,
						LastUpdateTime:     metav1.NewTime(now.Add(-time.Minute)),
						LastTransitionTime: metav1.NewTime(now.Add(-time.Minute)),
					},
				},
			},
			newCondition: getNewCondition(condType, v1.ConditionTrue, now.Add(time.Second)),
			expectedStatus: workloadv1alpha1.PartitionWorkloadStatus{
				Conditions: []workloadv1alpha1.PartitionWorkloadCondition{
					{
						Type:               condType,
						Status:             v1.ConditionTrue,
						LastUpdateTime:     metav1.NewTime(now.Add(time.Second)),
						LastTransitionTime: metav1.NewTime(now.Add(-time.Minute)),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetCondition(&tt.initialStatus, *tt.newCondition)
			if len(tt.initialStatus.Conditions) != len(tt.expectedStatus.Conditions) {
				t.Errorf("Condition count mismatch: got %d, want %d", len(tt.initialStatus.Conditions), len(tt.expectedStatus.Conditions))
			}
			if len(tt.initialStatus.Conditions) > 0 {
				got := tt.initialStatus.Conditions[0]
				want := tt.expectedStatus.Conditions[0]
				if got.Type != want.Type || got.Status != want.Status {
					t.Errorf("Condition mismatch: got %+v, want %+v", got, want)
				}
			}
		})
	}
}
