package controller

import (
	"context"
	"time"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	klog "k8s.io/klog/v2"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	history "k8s.io/kubernetes/pkg/controller/history"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	condition "github.com/2170chm/k8s-partition-workload/internal/controller/condition"
	"github.com/2170chm/k8s-partition-workload/internal/controller/config"
	revision "github.com/2170chm/k8s-partition-workload/internal/controller/revision"
	status "github.com/2170chm/k8s-partition-workload/internal/controller/status"
	sync "github.com/2170chm/k8s-partition-workload/internal/controller/sync"
	general "github.com/2170chm/k8s-partition-workload/internal/util/general"
	refmanager "github.com/2170chm/k8s-partition-workload/internal/util/refmanager"
)

// PartitionWorkloadReconciler reconciles a PartitionWorkload object
type PartitionWorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	HistoryControl history.Interface
	SyncControl    sync.Interface
	StatusUpdater  status.Interface
}

// +kubebuilder:rbac:groups=workload.scott.dev,resources=partitionworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workload.scott.dev,resources=partitionworkloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workload.scott.dev,resources=partitionworkloads/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PartitionWorkloadReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	startTime := time.Now()
	defer func() {
		klog.InfoS("Finished syncing partitionworkload", "partitionworkload", request, "duration", time.Since(startTime))
	}()

	// Fetch the resource instance
	instance := &workloadv1alpha1.PartitionWorkload{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.InfoS("PartitionWorkload has been deleted", "partitionworkload", request)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.Selector)
	if err != nil {
		klog.ErrorS(err, "Error converting PartitionWorkload selector", "partitionworkload", request)
		return reconcile.Result{}, nil
	}

	// List all active Pods
	activePods, err := r.getActivePods(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	// klog.InfoS("---- pods update ----")
	// klog.InfoS("All activePods", "detail", klog.KObjSlice(activePods))

	// Claim/release pod ownership using label selector matching
	// This adopts pods that match our selector but aren't owned, and releases pods that don't match
	claimedPods, err := r.claimPods(instance, r.Scheme, activePods)
	if err != nil {
		return reconcile.Result{}, err
	}

	klog.InfoS("---- pods update ----")
	klog.InfoS("Claimed pods", "detail", klog.KObjSlice(claimedPods))

	// Revisions
	revisions, err := r.HistoryControl.ListControllerRevisions(instance, selector)
	if err != nil {
		return reconcile.Result{}, err
	}

	// sort for calculating next revision
	history.SortControllerRevisions(revisions)

	klog.InfoS("---- revision update ----")
	klog.InfoS("Sorted Revisions", "detail", klog.KObjSlice(revisions))

	currentRevision, updateRevision, collisionCount, err := r.getActiveRevisions(instance, revisions)
	if err != nil {
		return reconcile.Result{}, err
	}

	klog.InfoS("---- revision update ----")
	klog.InfoS("currentRevision", "detail", klog.KObj(currentRevision))
	klog.InfoS("updatedRevision", "detail", klog.KObj(updateRevision))
	klog.InfoS("collisionCount", "count", collisionCount)

	newStatus := workloadv1alpha1.PartitionWorkloadStatus{
		ObservedGeneration: instance.Generation,
		CurrentRevision:    currentRevision.Name,
		UpdateRevision:     updateRevision.Name,
		CollisionCount:     &collisionCount,
	}

	// Core logic to scale and update pods
	syncErr := r.syncPods(instance, &newStatus, currentRevision, updateRevision, claimedPods)

	// Update the status of the resource
	if err = r.StatusUpdater.UpdateStatus(instance, &newStatus, claimedPods); err != nil {
		return reconcile.Result{}, err
	}

	// Clean up history that's above of the limit
	if err = r.truncateHistory(claimedPods, revisions, currentRevision, updateRevision); err != nil {
		klog.ErrorS(err, "Failed to truncate history for PartitionWorkload", "PartitionWorkload", request)
	}

	if syncErr != nil {
		klog.InfoS("---- sync error ----")
		klog.ErrorS(syncErr, "Failed to sync pods for PartitionWorkload", "PartitionWorkload", request)
	}

	klog.InfoS("Successfully reconciled without errors")
	// Return the syncErr. If there is a syncErr, controller will requeue
	return reconcile.Result{}, syncErr
}

// SetupWithManager sets up the controller with the Manager.
func (r *PartitionWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workloadv1alpha1.PartitionWorkload{}).
		Named("partitionworkload").
		Complete(r)
}

func (r *PartitionWorkloadReconciler) getActivePods(instance *workloadv1alpha1.PartitionWorkload) ([]*v1.Pod, error) {
	podList := &v1.PodList{}
	if err := r.Client.List(context.TODO(), podList, client.InNamespace(instance.Namespace)); err != nil {
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

func (r *PartitionWorkloadReconciler) claimPods(instance *workloadv1alpha1.PartitionWorkload, scheme *runtime.Scheme, pods []*v1.Pod) ([]*v1.Pod, error) {
	mgr, err := refmanager.NewRefManager(r.Client, instance.Spec.Selector, instance, scheme)
	if err != nil {
		return nil, err
	}
	selected := make([]metav1.Object, len(pods))
	for i, pod := range pods {
		selected[i] = pod
	}

	claimed, err := mgr.ClaimOwnedObjects(selected)
	if err != nil {
		return nil, err
	}

	claimedPods := make([]*v1.Pod, len(claimed))
	for i, pod := range claimed {
		claimedPods[i] = pod.(*v1.Pod)
	}

	return claimedPods, nil
}

func (r *PartitionWorkloadReconciler) getActiveRevisions(instance *workloadv1alpha1.PartitionWorkload, revisions []*apps.ControllerRevision) (
	*apps.ControllerRevision, *apps.ControllerRevision, int32, error,
) {
	var currentRevision, updateRevision *apps.ControllerRevision
	revisionCount := len(revisions)

	// Use a local copy of instance.Status.CollisionCount to avoid modifying instance.Status directly.
	// CollisionCount tracks hash collisions when creating revision names
	var collisionCount int32
	if instance.Status.CollisionCount != nil {
		collisionCount = *instance.Status.CollisionCount
	}

	// Calculate the next revision number
	var nextRevision int64
	revisionCount = len(revisions)
	if revisionCount <= 0 {
		nextRevision = 1
	} else {
		nextRevision = revisions[revisionCount-1].Revision + 1
	}

	// Create a new revision representing the current spec's pod template
	// If an equivalent revision already exists, it will be reused
	updateRevision, err := revision.NewRevision(instance, nextRevision, &collisionCount)
	if err != nil {
		return nil, nil, collisionCount, err
	}

	// Check if equivalent revision exists
	equalRevisions := history.FindEqualRevisions(revisions, updateRevision)
	equalCount := len(equalRevisions)

	if equalCount > 0 && history.EqualRevision(revisions[revisionCount-1], equalRevisions[equalCount-1]) {
		// if the equivalent revision is immediately prior the update revision has not changed
		updateRevision = revisions[revisionCount-1]
	} else if equalCount > 0 {
		// if the equivalent revision is not immediately prior we will roll back by incrementing the
		// Revision of the equivalent revision
		updateRevision, err = r.HistoryControl.UpdateControllerRevision(equalRevisions[equalCount-1], updateRevision.Revision)
		if err != nil {
			return nil, nil, collisionCount, err
		}
	} else {
		// if there is no equivalent revision we create a new one
		updateRevision, err = r.HistoryControl.CreateControllerRevision(instance, updateRevision, &collisionCount)
		if err != nil {
			return nil, nil, collisionCount, err
		}
	}

	// attempt to find the revision that corresponds to the current revision
	for i := range revisions {
		if revisions[i].Name == instance.Status.CurrentRevision {
			currentRevision = revisions[i]
			break
		}
	}

	// if the current revision is nil we initialize the history by setting it to the update revision
	if currentRevision == nil {
		currentRevision = updateRevision
	}

	return currentRevision, updateRevision, collisionCount, nil
}

func (r *PartitionWorkloadReconciler) syncPods(
	instance *workloadv1alpha1.PartitionWorkload, newStatus *workloadv1alpha1.PartitionWorkloadStatus,
	currentRevision, updateRevision *apps.ControllerRevision, pods []*v1.Pod,
) error {
	// If PartitionWorkload is being deleted, just let garbage collection clean up pods
	if instance.DeletionTimestamp != nil {
		return nil
	}

	// ApplyRevision reconstructs the PartitionWorkload spec as it was at each revision
	// currentSet = PartitionWorkload with currentRevision's pod template
	// updateSet = PartitionWorkload with updateRevision's pod template (latest spec)
	// This lets us compare and transition pods between versions
	currentPW, err := revision.ApplyRevision(instance, currentRevision)
	if err != nil {
		return err
	}
	updatedPW, err := revision.ApplyRevision(instance, updateRevision)
	if err != nil {
		return err
	}

	klog.InfoS("---- partition workload definition update ----")
	klog.InfoS("currentPW definition", "detail", currentPW)
	klog.InfoS("updatedPW definition", "detail", updatedPW)

	// Scale operation: Adjust the number of pods to match spec.Replicas
	// Creates new pods or deletes existing ones without changing their template version
	// Returns scaling=true if scale operation is in progress (skip updates until stable)
	err = r.SyncControl.ScaleAndUpdate(currentPW, updatedPW, currentRevision.Name, updateRevision.Name, pods)
	if err != nil {
		cond := workloadv1alpha1.PartitionWorkloadCondition{
			Type:               workloadv1alpha1.PartionWorkloadConditionFailedScale,
			Status:             v1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Message:            err.Error(),
		}
		condition.SetCondition(newStatus, cond)
	}

	return err
}

// truncateHistory truncates any non-live ControllerRevisions in revisions from pw's history. The UpdateRevision and
// CurrentRevision in pw's Status are considered to be live. Any revisions associated with the Pods in pods are also
// considered to be live. Non-live revisions are deleted, starting with the revision with the lowest Revision, until
// only RevisionHistoryLimit revisions remain. If the returned error is nil the operation was successful. This method
// expects that revisions is sorted when supplied.
//
// Live revisions = revisions actively used by pods or tracked in status
// Historic revisions = old unused revisions that can be garbage collected
// This prevents unbounded growth of ControllerRevision objects
func (r *PartitionWorkloadReconciler) truncateHistory(
	pods []*v1.Pod,
	revisions []*apps.ControllerRevision,
	current *apps.ControllerRevision,
	update *apps.ControllerRevision,
) error {
	nonLiveRevisions := make([]*apps.ControllerRevision, 0, len(revisions))

	// Identify which revisions are still in use:
	// 1. Current/update revisions (in status)
	// 2. Revisions that any existing pod is running
	// All others are candidates for deletion
	for i := range revisions {
		if revisions[i].Name != current.Name && revisions[i].Name != update.Name {
			var found bool
			for _, pod := range pods {
				if general.EqualToRevisionHash(pod, revisions[i].Name) {
					found = true
					break
				}
			}
			if !found {
				nonLiveRevisions = append(nonLiveRevisions, revisions[i])
			}
		}
	}

	// Note that the historySize is the max number of non-live revisions allowed
	// A live revision is a revision that is either being used by at least one
	// pod or is the updaterevision or the currenrevision of PartitionWorkload
	// It does not represent the total number of controllerrevisions
	historySize := len(nonLiveRevisions)
	historyLimit := config.DefaultHistoryLimit

	klog.InfoS("---- truncate history ----")
	klog.InfoS("Calculated history metrics", "history size", historySize, "history limit", historyLimit)

	if historySize <= historyLimit {
		return nil
	}

	// Delete oldest non-live revisions first (array is sorted oldest to newest)
	// Keep only the most recent 'historyLimit' revisions for potential rollback
	nonLiveRevisions = nonLiveRevisions[:(historySize - historyLimit)]
	for i := 0; i < len(nonLiveRevisions); i++ {
		klog.InfoS("Deleting revision", "revision", klog.KObj(nonLiveRevisions[i]))
		if err := r.HistoryControl.DeleteControllerRevision(nonLiveRevisions[i]); err != nil {
			return err
		}
	}
	return nil
}
