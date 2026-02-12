package sync

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
	klog "k8s.io/klog/v2"
)

const (
	// LengthOfInstanceID is the length of instance-id
	LengthOfInstanceID = 5

	// When batching pod creates, initialBatchSize is the size of the initial batch.
	initialBatchSize = 1

	notEnoughPodsWithUpdatedRevisionToDeleteErrString = "Not enough pods with the updated revision to delete"

	notEnoughPodsWithCurrentRevisionToDeleteErrString = "Not enough pods with the current revision to delete"
)

// Scale manages the scaling of pods to match the desired replica count and pod versions
// based on partition and template
//
// Parameters:
// - currentPW: PartionWorkload reconstructed with OLD pod template (from currentRevision)
// - updatePW: PartitionWorkload reconstructed with NEW pod template (from updatedRevision)
// - currentRevision: Name of the stable revision (for old-version pods)
// - updatedRevision: Name of the target revision (for new-version pods)
// - pods: All active pods owned by this PartitionWorkload
//
// Returns:
// - error: any error encountered during scaling
func (r *realSync) ScaleAndUpdate(
	currentPW, updatedPW *workloadv1alpha1.PartitionWorkload,
	currentRevision, updatedRevision string,
	pods []*v1.Pod,
) error {
	// Validate that replicas field is set
	if updatedPW.Spec.Replicas == nil {
		return fmt.Errorf("spec.Replicas is nil")
	}

	// PHASE 2: Calculate how many pods (of each version) need to be created or deleted
	// calculateDiffsWithExpectation computes:
	// - scaleUpNum: how many pods to create
	// - scaleUpNumOldRevision: how many of the new pods should use currentRevision (old template)
	// - scaleDownNum: how many pods with the latest versions to delete
	// - scaleDownNumOldRevision: how many pods with the current versions to delete
	// This calculation includes both replica scaling and updates (updates are recreate updates so essentially
	// it is the same as deleting a pod of a version and creating a pod with the target version). Hence the
	// four scale nums can represent the final desired state.
	// It supports rollbacks as well if, for example, pw.spec.partition is decremented.
	diffRes := calculateDiffs(updatedPW, pods, currentRevision, updatedRevision)
	klog.InfoS("---- calculated pod diffs----")
	klog.InfoS("calculated diffs", "detail", diffRes)

	// Group pods by whether they're on the new revision or not
	// updatedPods: pods already on updatedRevision
	// notUpdatedPods: pods still on older revisions
	updatedPods, notUpdatedPods := groupUpdatedAndNotUpdatedPods(pods, updatedRevision)
	klog.InfoS("---- pods grouped by revision ----")
	klog.InfoS("Pods with updated revision", "detail", klog.KObjSlice(updatedPods))
	klog.InfoS("Pods with other revisions", "detail", klog.KObjSlice(notUpdatedPods))

	// PHASE 3: Scale up - create new pods if we need more replicas
	// Create the pods (some with old template, some with new)
	if err := r.createPods(diffRes.scaleUpNum, diffRes.scaleUpNumOldRevision,
		currentPW, updatedPW, currentRevision, updatedRevision); err != nil {
		return err
	}

	// PHASE 4: Scale down - delete excess pods when replicas is reduced
	if err := r.deletePods(diffRes.scaleDownNum, diffRes.scaleDownNumOldRevision, updatedPods, notUpdatedPods); err != nil {
		return err
	}

	return nil
}

// createPods creates a batch of new pods for scale-out
// Some pods may use the old template (currentRevision), others use new template (updatedRevision)
// This handles partition logic: maintaining both old and new versions during updates
//
// Parameters:
// - expectedCurrentCreations: total number of current pods to create (current revision)
// - expectedUpdatedCreations: total number of updated pods to create (updated revision)
// - currentPW: PartitionWorkload with old pod template
// - updatedPW: PartitionWorkload with new pod template
// - currentRevision, updatedRevision: revision names for labeling pods
//
// Returns:
// - error: any error encountered (may be partial success)
func (r *realSync) createPods(
	expectedUpdatedCreations, expectedCurrentCreations int,
	currentPW, updatedPW *workloadv1alpha1.PartitionWorkload,
	currentRevision, updatedRevision string,
) error {
	if expectedCurrentCreations == 0 && expectedUpdatedCreations == 0 {
		return nil
	}
	// Generate pod specifications for all pods we need to create
	// This returns a mix of pods: some with old template, some with new template
	newPods, err := newMultiVersionedPods(currentPW, updatedPW, currentRevision, updatedRevision, expectedCurrentCreations, expectedUpdatedCreations)
	if err != nil {
		return err
	}

	klog.InfoS("---- pod creation plan ----")
	klog.InfoS("expectedCurrentCreations", "count", expectedCurrentCreations)
	klog.InfoS("expectedUpdatedCreations", "count", expectedUpdatedCreations)
	klog.InfoS("pods object to create", "detail", klog.KObjSlice(newPods))

	podsCreationChan := make(chan *v1.Pod, len(newPods))
	for _, p := range newPods {
		podsCreationChan <- p
	}

	// Create pods slowly in batches to avoid overwhelming the API server
	// DoItSlowly uses exponential backoff for batch sizes
	_, err = generalutil.DoItSlowly(len(newPods), initialBatchSize, func() error {
		pod := <-podsCreationChan
		if createErr := r.Create(context.TODO(), pod); createErr != nil {
			return createErr
		}
		return nil
	})

	return err
}

// deletePods deletes a batch of pods

// Returns:
// - error: any error encountered
func (r *realSync) deletePods(expectedUpdatedDeletions int, expectedCurrentDeletions int, updatedPods, notUpdatedPods []*v1.Pod) error {
	if expectedUpdatedDeletions == 0 && expectedCurrentDeletions == 0 {
		return nil
	}

	var podsToDelete []*v1.Pod
	if expectedUpdatedDeletions > 0 {
		if len(updatedPods) < expectedUpdatedDeletions {
			return fmt.Errorf(notEnoughPodsWithUpdatedRevisionToDeleteErrString)
		}
		sortPodsOldestFirst(updatedPods)
		podsToDelete = append(podsToDelete, updatedPods[:expectedUpdatedDeletions]...)
	}

	if expectedCurrentDeletions > 0 {
		if len(notUpdatedPods) < expectedCurrentDeletions {
			return fmt.Errorf(notEnoughPodsWithCurrentRevisionToDeleteErrString)
		}
		sortPodsOldestFirst(notUpdatedPods)
		podsToDelete = append(podsToDelete, notUpdatedPods[:expectedCurrentDeletions]...)
	}

	klog.InfoS("---- pod deletion plan ----")
	klog.InfoS("expectedCurrentDeletions", "count", expectedCurrentDeletions)
	klog.InfoS("expectedUpdatedDeletions", "count", expectedUpdatedDeletions)
	klog.InfoS("pods object to delete", "pod", klog.KObjSlice(podsToDelete))

	for _, pod := range podsToDelete {
		if err := r.Delete(context.TODO(), pod); err != nil {
			return err
		}
	}

	return nil
}
