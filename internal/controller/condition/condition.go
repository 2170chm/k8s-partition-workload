package condition

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
)

func SetCondition(status *workloadv1alpha1.PartitionWorkloadStatus, condition workloadv1alpha1.PartitionWorkloadCondition) {
	currentCond := GetCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}

	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}

	newConditions := filterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

func GetCondition(status workloadv1alpha1.PartitionWorkloadStatus, condType workloadv1alpha1.PartitionWorkloadConditionType) *workloadv1alpha1.PartitionWorkloadCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func filterOutCondition(conditions []workloadv1alpha1.PartitionWorkloadCondition, condType workloadv1alpha1.PartitionWorkloadConditionType) []workloadv1alpha1.PartitionWorkloadCondition {
	var newConditions []workloadv1alpha1.PartitionWorkloadCondition

	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}

	return newConditions
}
