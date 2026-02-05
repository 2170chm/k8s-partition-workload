/*
Copyright 2021 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	config "github.com/2170chm/k8s-partition-workload/internal/controller/config"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
	apps "k8s.io/api/apps/v1"
)

type expectationDiffs struct {
	// scaleUpNum is a non-negative integer, which indicates the number that should scale up.
	scaleUpNum int
	// scaleNumOldRevision is a non-negative integer, which indicates the number of old revision Pods that should scale up.
	// It might be bigger than scaleUpNum, but controller will scale up at most scaleUpNum number of Pods.
	scaleUpNumOldRevision int
	// scaleDownNum is a non-negative integer, which indicates the number that should scale down.
	scaleDownNum int
	// scaleDownNumOldRevision is a non-negative integer, which indicates the number of old revision Pods that should scale down.
	// It might be bigger than scaleDownNum, but controller will scale down at most scaleDownNum number of Pods.
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
		if equalToRevisionHash(p, updatedRevision) {
			newRevisionCount++
		} else {
			oldRevisionCount++
		}
	}

	updateNewDiff := newRevisionCount - partition
	updateOldDiff := oldRevisionCount - (replicas - partition)

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
		if equalToRevisionHash(p, updatedRevision) {
			update = append(update, p)
		} else {
			notUpdate = append(notUpdate, p)
		}
	}
	return
}

func equalToRevisionHash(pod *v1.Pod, hash string) bool {
	return pod.GetLabels()[apps.ControllerRevisionHashLabelKey] == hash
}

func newMultiVersionedPods(currentPW, updatedPW *workloadv1alpha1.PartitionWorkload,
	currentRevision, updatedRevision string,
	expectedCurrentCreations, expectedUpdatedCreations int,
) ([]*v1.Pod, error) {
	var newPods []*v1.Pod
	if expectedCurrentCreations > 0 {
		newPods = append(newPods, newVersionedPods(currentPW, currentRevision, expectedCurrentCreations)...)
	}
	if expectedUpdatedCreations > 0 {
		newPods = append(newPods, newVersionedPods(updatedPW, updatedRevision, expectedUpdatedCreations)...)
	}
	return newPods, nil
}

func newVersionedPods(pw *workloadv1alpha1.PartitionWorkload, revision string, replicas int) []*v1.Pod {
	var newPods []*v1.Pod
	for i := 0; i < replicas; i++ {

		pod, _ := kubecontroller.GetPodFromTemplate(&pw.Spec.Template, pw, metav1.NewControllerRef(pw, config.ControllerKind))
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		writeRevisionHash(pod, revision)

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
