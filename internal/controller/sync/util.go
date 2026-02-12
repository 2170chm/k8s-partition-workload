package sync

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/utils/integer"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	kubecontroller "k8s.io/kubernetes/pkg/controller"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
	apps "k8s.io/api/apps/v1"
)

type expectationDiffs struct {
	// scaleUpNum is a non-negative integer, which indicates the number of updated revision pods that should scale up.
	scaleUpNum int
	// scaleNumOldRevision is a non-negative integer, which indicates the number of current revision Pods that should scale up.
	scaleUpNumOldRevision int
	// scaleDownNum is a non-negative integer, which indicates the number of updated revision pods that should scale down.
	scaleDownNum int
	// scaleDownNumOldRevision is a non-negative integer, which indicates the number of current revision Pods that should scale down.
	scaleDownNumOldRevision int
}

func (e expectationDiffs) isEmpty() bool {
	return reflect.DeepEqual(e, expectationDiffs{})
}

// String implement this to print information in klog
func (e expectationDiffs) String() string {
	return fmt.Sprintf("{scaleUpNum:%d scaleUpNumOldRevision:%d scaleDownNum:%d scaleDownNumOldRevision:%d}",
		e.scaleUpNum, e.scaleUpNumOldRevision, e.scaleDownNum, e.scaleDownNumOldRevision)
}

func calculateDiffs(pw *workloadv1alpha1.PartitionWorkload, pods []*v1.Pod, currentRevision, updatedRevision string) (res expectationDiffs) {
	replicas := int(*pw.Spec.Replicas)
	var partition int

	// Partition defaults to replicas. If specified partition is greater than replicas, clamp it to replicas
	if pw.Spec.Partition != nil {
		partition = integer.IntMin(int(*pw.Spec.Partition), replicas)
	} else {
		partition = replicas
	}

	var newRevisionCount, oldRevisionCount int

	defer func() {
		if res.isEmpty() {
			return
		}
		klog.InfoS("---- pod diff details ----")
		klog.InfoS("Calculate diffs for PartitionWorkload", "PartitionWorkload", klog.KObj(pw), "replicas", replicas, "partition", partition, "allPodCount", len(pods), "newRevisionCount", newRevisionCount,
			"oldrevisionCount", oldRevisionCount,
			"result", res)
	}()

	for _, p := range pods {
		if generalutil.EqualToRevisionHash(p, updatedRevision) {
			klog.InfoS("---- pod diff details ----")
			klog.InfoS("Pod revision is the updated revision", "pod revision hash", p.GetLabels()[apps.ControllerRevisionHashLabelKey], "updated revision being compared to", updatedRevision)
			newRevisionCount++
		} else {
			klog.InfoS("Pod revision is not the updated revision", "pod revision hash", p.GetLabels()[apps.ControllerRevisionHashLabelKey], "updated revision being compared to", updatedRevision)
			oldRevisionCount++
		}
	}

	updateNewDiff := newRevisionCount - partition
	updateOldDiff := oldRevisionCount - (replicas - partition)

	// If only one revision exists, sync to <replicas> number of pods with updatedRevision.
	// This is because one when there is only one revision, all pods are grouped to updatedPods,
	// So it would cause an error if we want to scale down current revision pods
	if updatedRevision == currentRevision {
		klog.InfoS("Only one revision detected, updating with latest revision to match replica count")
		updateNewDiff = newRevisionCount + oldRevisionCount - replicas
		updateOldDiff = 0
	}

	klog.InfoS("Update Diffs and revisions", "current revision diff", updateOldDiff, "new revision diff", updateNewDiff, "current revision name", currentRevision, "updatedRevision name", updatedRevision)

	// scale up
	res.scaleUpNum = 0
	res.scaleUpNumOldRevision = 0
	if updateNewDiff < 0 {
		res.scaleUpNum += generalutil.Abs(updateNewDiff)
	}
	if updateOldDiff < 0 {
		res.scaleUpNumOldRevision += generalutil.Abs(updateOldDiff)
	}

	// scale down
	res.scaleDownNum = 0
	res.scaleDownNumOldRevision = 0
	if updateNewDiff > 0 {
		res.scaleDownNum += generalutil.Abs(updateNewDiff)
	}
	if updateOldDiff > 0 {
		res.scaleDownNumOldRevision += generalutil.Abs(updateOldDiff)
	}

	return
}

func groupUpdatedAndNotUpdatedPods(pods []*v1.Pod, updatedRevision string) (update, notUpdate []*v1.Pod) {
	for _, p := range pods {
		if generalutil.EqualToRevisionHash(p, updatedRevision) {
			update = append(update, p)
		} else {
			notUpdate = append(notUpdate, p)
		}
	}
	return
}

func newMultiVersionedPods(currentPW, updatedPW *workloadv1alpha1.PartitionWorkload,
	currentRevision, updatedRevision string,
	expectedCurrentCreations, expectedUpdatedCreations int,
) ([]*v1.Pod, error) {
	var newPods []*v1.Pod
	if expectedCurrentCreations > 0 {
		newPods = append(newPods, NewVersionedPods(currentPW, currentRevision, expectedCurrentCreations)...)
	}
	if expectedUpdatedCreations > 0 {
		newPods = append(newPods, NewVersionedPods(updatedPW, updatedRevision, expectedUpdatedCreations)...)
	}
	return newPods, nil
}

func NewVersionedPods(pw *workloadv1alpha1.PartitionWorkload, revision string, replicas int) []*v1.Pod {
	var newPods []*v1.Pod
	for i := 0; i < replicas; i++ {
		pod, _ := kubecontroller.GetPodFromTemplate(&pw.Spec.Template, pw, metav1.NewControllerRef(pw, workloadv1alpha1.SchemeGroupVersion.WithKind("PartitionWorkload")))
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}

		// Write revision hash to labels for revision management
		writeRevisionHash(pod, revision)

		// Let k8s generate random name
		pod.GenerateName = fmt.Sprintf("%s-", pw.Name)
		pod.Namespace = pw.Namespace

		newPods = append(newPods, pod)
	}

	return newPods
}

func writeRevisionHash(obj metav1.Object, hash string) {
	if obj.GetLabels() == nil {
		obj.SetLabels(make(map[string]string, 1))
	}
	// Controller-revision-hash defaults to be "{PartitionWorkload_NAME}-{HASH}",
	// pod-template-hash should always be the short format.
	obj.GetLabels()[apps.ControllerRevisionHashLabelKey] = hash
	obj.GetLabels()[apps.DefaultDeploymentUniqueLabelKey] = getShortHash(hash)
}

func getShortHash(hash string) string {
	// This makes sure the real hash must be the last '-' substring of revision name
	list := strings.Split(hash, "-")
	return list[len(list)-1]
}

func sortPodsOldestFirst(pods []*v1.Pod) {
	generalutil.SortPods(pods, func(i, j int) bool {
		return pods[i].CreationTimestamp.Time.Before(
			pods[j].CreationTimestamp.Time,
		)
	})
}
