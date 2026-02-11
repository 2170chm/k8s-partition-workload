package sync

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
	"k8s.io/apimachinery/pkg/types"
)

var (
	kscheme *runtime.Scheme
)

const (
	testKind         = "PartitionWorkload"
	testAPIVersion   = "workload.scott.dev/v1alpha1"
	testPWName       = "test-pw"
	testPodName      = "test-pod"
	testUID          = types.UID("test")
	revision1        = "1"
	revision2        = "2"
	testImage        = "nginx"
	testUpdatedImage = "nginx2"
)

func init() {
	kscheme = runtime.NewScheme()
	utilruntime.Must(workloadv1alpha1.AddToScheme(kscheme))
	utilruntime.Must(corev1.AddToScheme(kscheme))
}

func TestCreatePods(t *testing.T) {
	r := newFakeControl()
	currentPW := getPW(2)
	updatePW := currentPW.DeepCopy()

	err := r.createPods(
		1,
		1,
		currentPW,
		updatePW,
		revision1,
		revision2,
	)
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}

	pods := v1.PodList{}
	if err := r.List(context.TODO(), &pods, client.InNamespace("default")); err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}

	sort.Slice(pods.Items, func(i, j int) bool {
		return pods.Items[i].Labels[apps.ControllerRevisionHashLabelKey] < pods.Items[j].Labels[apps.ControllerRevisionHashLabelKey]
	})

	expectedPods := []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    v1.NamespaceDefault,
				Name:         pods.Items[0].Name,
				GenerateName: testPWName + "-",
				Labels: map[string]string{
					apps.ControllerRevisionHashLabelKey:  revision1,
					apps.DefaultDeploymentUniqueLabelKey: revision1,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         testAPIVersion,
						Kind:               testKind,
						Name:               testPWName,
						UID:                testUID,
						Controller:         func() *bool { a := true; return &a }(),
						BlockOwnerDeletion: func() *bool { a := true; return &a }(),
					},
				},
				ResourceVersion: "1",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  testImage,
						Image: testImage,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    v1.NamespaceDefault,
				Name:         pods.Items[1].Name,
				GenerateName: testPWName + "-",
				Labels: map[string]string{
					apps.ControllerRevisionHashLabelKey:  revision2,
					apps.DefaultDeploymentUniqueLabelKey: revision2,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         testAPIVersion,
						Kind:               testKind,
						Name:               testPWName,
						UID:                testUID,
						Controller:         func() *bool { a := true; return &a }(),
						BlockOwnerDeletion: func() *bool { a := true; return &a }(),
					},
				},
				ResourceVersion: "1",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  testImage,
						Image: testImage,
					},
				},
			},
		},
	}

	if len(pods.Items) != len(expectedPods) {
		t.Fatalf("expected pods \n%s\ngot pods\n%s", generalutil.DumpJSON(expectedPods), generalutil.DumpJSON(pods.Items))
	}

	if !reflect.DeepEqual(expectedPods, pods.Items) {
		t.Fatalf("expected pods \n%s\ngot pods\n%s", generalutil.DumpJSON(expectedPods), generalutil.DumpJSON(pods.Items))
	}
}

func TestDeletePods(t *testing.T) {
	podsToDelete := []*v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    v1.NamespaceDefault,
				GenerateName: testPWName + "-",
				Labels: map[string]string{
					apps.ControllerRevisionHashLabelKey:  revision1,
					apps.DefaultDeploymentUniqueLabelKey: revision1,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         testAPIVersion,
						Kind:               testKind,
						Name:               testPWName,
						UID:                testUID,
						Controller:         func() *bool { a := true; return &a }(),
						BlockOwnerDeletion: func() *bool { a := true; return &a }(),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    v1.NamespaceDefault,
				GenerateName: testPWName + "-",
				Labels: map[string]string{
					apps.ControllerRevisionHashLabelKey:  revision2,
					apps.DefaultDeploymentUniqueLabelKey: revision2,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         testAPIVersion,
						Kind:               testKind,
						Name:               testPWName,
						UID:                testUID,
						Controller:         func() *bool { a := true; return &a }(),
						BlockOwnerDeletion: func() *bool { a := true; return &a }(),
					},
				},
			},
		},
	}

	r := newFakeControl()
	for _, p := range podsToDelete {
		_ = r.Create(context.TODO(), p)
	}

	updatedPods, notUpdatedPods := groupUpdatedAndNotUpdatedPods(podsToDelete, revision2)
	if len(updatedPods) != 1 || len(notUpdatedPods) != 1 {
		t.Fatalf("Incorrect grouping of updated and not updated pods - updated pods: %v, not updated pods: %v", updatedPods, notUpdatedPods)
	}
	err := r.deletePods(1, 1, updatedPods, notUpdatedPods)
	if err != nil {
		t.Fatalf("failed to delete pods: %v", err)
	}

	gotPods := v1.PodList{}
	if err := r.List(context.TODO(), &gotPods, client.InNamespace("default")); err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}
	if len(gotPods.Items) > 0 {
		t.Fatalf("expected no pods left, actually: %v", gotPods.Items)
	}

	err = r.deletePods(2, 1, updatedPods, notUpdatedPods)
	if err == nil || !strings.Contains(err.Error(), notEnoughPodsWithUpdatedRevisionToDeleteErrString) {
		t.Fatalf("failed to detect that not enough pods with updated revision can be deleted: %v", err)
	}

	err = r.deletePods(1, 2, updatedPods, notUpdatedPods)
	if err == nil || !strings.Contains(err.Error(), notEnoughPodsWithCurrentRevisionToDeleteErrString) {
		t.Fatalf("failed to detect that not enough pods with updated revision can be deleted: %v", err)
	}
}

func TestScaleAndUpdate(t *testing.T) {
	tests := []struct {
		name                       string
		getPartitionWorkload       func() [2]*workloadv1alpha1.PartitionWorkload
		getRevisions               func() [2]string
		getImageNames              func() [2]string
		getPods                    func() []*v1.Pod
		expectedPodsCnt            int
		expectedUpdatedRevisionCnt int
		expectedCurrentRevisionCnt int
	}{
		{
			name: "Scale down - PartitionWorkload(replicas=3,partition=nil), pods=5, and scale replicas 5 -> 3",
			getPartitionWorkload: func() [2]*workloadv1alpha1.PartitionWorkload {
				obj := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas: generalutil.Int32Ptr(3),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testImage,
										Image: testImage,
									},
								},
							},
						},
					},
				}
				return [2]*workloadv1alpha1.PartitionWorkload{obj.DeepCopy(), obj.DeepCopy()}
			},
			getRevisions: func() [2]string {
				return [2]string{revision1, revision1}
			},
			getImageNames: func() [2]string {
				return [2]string{testImage, testImage}
			},
			getPods: func() []*v1.Pod {
				obj := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision1,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testImage,
								Image: testImage,
							},
						},
					},
				}
				return generatePods(obj, 5)
			},
			expectedPodsCnt:            3,
			expectedCurrentRevisionCnt: 0,
			expectedUpdatedRevisionCnt: 3,
		},
		{
			name: "Scale up - PartitionWorkload(replicas=5,partition=nil), pods=3, and scale replicas 3 -> 5",
			getPartitionWorkload: func() [2]*workloadv1alpha1.PartitionWorkload {
				obj := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas: generalutil.Int32Ptr(5),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testImage,
										Image: testImage,
									},
								},
							},
						},
					},
				}
				return [2]*workloadv1alpha1.PartitionWorkload{obj.DeepCopy(), obj.DeepCopy()}
			},
			getRevisions: func() [2]string {
				return [2]string{revision1, revision1}
			},
			getImageNames: func() [2]string {
				return [2]string{testImage, testImage}
			},
			getPods: func() []*v1.Pod {
				obj := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision1,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testImage,
								Image: testImage,
							},
						},
					},
				}
				return generatePods(obj, 3)
			},
			expectedPodsCnt:            5,
			expectedCurrentRevisionCnt: 0,
			expectedUpdatedRevisionCnt: 5,
		},
		{
			name: "Full roll out update - PartitionWorkload(replicas=3,partition=3), pods=3, 3 old pods -> new pods",
			getPartitionWorkload: func() [2]*workloadv1alpha1.PartitionWorkload {
				pw1 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas: generalutil.Int32Ptr(3),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testImage,
										Image: testImage,
									},
								},
							},
						},
					},
				}
				pw2 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas: generalutil.Int32Ptr(3),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testUpdatedImage,
										Image: testUpdatedImage,
									},
								},
							},
						},
					},
				}
				return [2]*workloadv1alpha1.PartitionWorkload{pw1.DeepCopy(), pw2.DeepCopy()}
			},
			getRevisions: func() [2]string {
				return [2]string{revision1, revision2}
			},
			getImageNames: func() [2]string {
				return [2]string{testImage, testUpdatedImage}
			},
			getPods: func() []*v1.Pod {
				obj := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision1,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testImage,
								Image: testImage,
							},
						},
					},
				}
				return generatePods(obj, 3)
			},
			expectedPodsCnt:            3,
			expectedCurrentRevisionCnt: 0,
			expectedUpdatedRevisionCnt: 3,
		},
		{
			name: "Partitioned update - PartitionWorkload(replicas=3,partition=2), pods=3, 2 old pods -> new pods",
			getPartitionWorkload: func() [2]*workloadv1alpha1.PartitionWorkload {
				pw1 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas:  generalutil.Int32Ptr(3),
						Partition: generalutil.Int32Ptr(2),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testImage,
										Image: testImage,
									},
								},
							},
						},
					},
				}
				pw2 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas:  generalutil.Int32Ptr(3),
						Partition: generalutil.Int32Ptr(2),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testUpdatedImage,
										Image: testUpdatedImage,
									},
								},
							},
						},
					},
				}
				return [2]*workloadv1alpha1.PartitionWorkload{pw1.DeepCopy(), pw2.DeepCopy()}
			},
			getRevisions: func() [2]string {
				return [2]string{revision1, revision2}
			},
			getImageNames: func() [2]string {
				return [2]string{testImage, testUpdatedImage}
			},
			getPods: func() []*v1.Pod {
				obj := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision1,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testImage,
								Image: testImage,
							},
						},
					},
				}
				return generatePods(obj, 3)
			},
			expectedPodsCnt:            3,
			expectedCurrentRevisionCnt: 1,
			expectedUpdatedRevisionCnt: 2,
		},
		{
			name: "Partitioned roll back update - PartitionWorkload(replicas=3,partition=1), pods=3 (2 old 1 new), 1 new pod -> old pod",
			getPartitionWorkload: func() [2]*workloadv1alpha1.PartitionWorkload {
				pw1 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas: generalutil.Int32Ptr(3),
						// The old pw's partition is irrelevant to the rollback since we only calculate
						// the state based on the latest partition, but it is set to 2 for completeness
						// and to match the current pod state before this update
						Partition: generalutil.Int32Ptr(2),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testImage,
										Image: testImage,
									},
								},
							},
						},
					},
				}
				pw2 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas:  generalutil.Int32Ptr(3),
						Partition: generalutil.Int32Ptr(1),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testUpdatedImage,
										Image: testUpdatedImage,
									},
								},
							},
						},
					},
				}
				return [2]*workloadv1alpha1.PartitionWorkload{pw1.DeepCopy(), pw2.DeepCopy()}
			},
			getRevisions: func() [2]string {
				return [2]string{revision1, revision2}
			},
			getImageNames: func() [2]string {
				return [2]string{testImage, testUpdatedImage}
			},
			getPods: func() []*v1.Pod {
				pod1 := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision1,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testImage,
								Image: testImage,
							},
						},
					},
				}
				pod2 := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision2,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testUpdatedImage,
								Image: testUpdatedImage,
							},
						},
					},
				}
				pods := generatePods(pod1, 1)
				pods = append(pods, generatePods(pod2, 2)...)
				return pods
			},
			expectedPodsCnt:            3,
			expectedCurrentRevisionCnt: 2,
			expectedUpdatedRevisionCnt: 1,
		},
		{
			name: "Partitioned update and scale up - PartitionWorkload(replicas=4,partition=3), pods=3 (2 new), create 1 new pod",
			getPartitionWorkload: func() [2]*workloadv1alpha1.PartitionWorkload {
				pw1 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas:  generalutil.Int32Ptr(3),
						Partition: generalutil.Int32Ptr(2),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testImage,
										Image: testImage,
									},
								},
							},
						},
					},
				}
				pw2 := &workloadv1alpha1.PartitionWorkload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testPWName,
						Namespace: v1.NamespaceDefault,
					},
					Spec: workloadv1alpha1.PartitionWorkloadSpec{
						Replicas:  generalutil.Int32Ptr(4),
						Partition: generalutil.Int32Ptr(3),
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  testUpdatedImage,
										Image: testUpdatedImage,
									},
								},
							},
						},
					},
				}
				return [2]*workloadv1alpha1.PartitionWorkload{pw1.DeepCopy(), pw2.DeepCopy()}
			},
			getRevisions: func() [2]string {
				return [2]string{revision1, revision2}
			},
			getImageNames: func() [2]string {
				return [2]string{testImage, testUpdatedImage}
			},
			getPods: func() []*v1.Pod {
				pod1 := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision1,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testImage,
								Image: testImage,
							},
						},
					},
				}
				pod2 := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPodName,
						Labels: map[string]string{
							apps.ControllerRevisionHashLabelKey: revision2,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  testUpdatedImage,
								Image: testUpdatedImage,
							},
						},
					},
				}
				pods := generatePods(pod1, 1)
				pods = append(pods, generatePods(pod2, 2)...)
				return pods
			},
			expectedPodsCnt:            4,
			expectedCurrentRevisionCnt: 1,
			expectedUpdatedRevisionCnt: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeControl()
			pods := tt.getPods()
			for _, pod := range pods {
				err := r.Create(context.TODO(), pod)
				if err != nil {
					t.Fatalf("failed to creat pods - error: %v", err)
				}
			}
			partitionWorkloads := tt.getPartitionWorkload()
			revisions := tt.getRevisions()
			imageNames := tt.getImageNames()
			currentPW := partitionWorkloads[0]
			updatedPW := partitionWorkloads[1]
			currentRevision := revisions[0]
			updatedRevision := revisions[1]
			currentImageName := imageNames[0]
			updatedImageName := imageNames[1]

			err := r.ScaleAndUpdate(currentPW, updatedPW, currentRevision, updatedRevision, pods)
			if err != nil {
				t.Fatalf("ScaleAndUpdate fails - error: %v", err)
			}
			podList := &v1.PodList{}
			err = r.List(context.TODO(), podList, &client.ListOptions{})
			if err != nil {
				t.Fatalf("Failed to list pods - error: %v", err)
			}

			currentRevisionCnt := 0
			updatedRevisionCnt := 0
			for _, pod := range podList.Items {
				if pod.Labels[apps.ControllerRevisionHashLabelKey] == updatedRevision {
					if pod.Spec.Containers[0].Name != updatedImageName {
						t.Fatalf("Pod controller revision hash label key and image name doesn't match - Revision: %s, Image name: %s", pod.Labels[apps.ControllerRevisionHashLabelKey], pod.Spec.Containers[0].Name)
					}
					updatedRevisionCnt += 1
				} else {
					if pod.Spec.Containers[0].Name != currentImageName {
						t.Fatalf("Pod controller revision hash label key and image name doesn't match - Revision: %s, Image name: %s", pod.Labels[apps.ControllerRevisionHashLabelKey], pod.Spec.Containers[0].Name)
					}
					currentRevisionCnt += 1
				}
			}
			totalPodCnt := currentRevisionCnt + updatedRevisionCnt
			if totalPodCnt != tt.expectedPodsCnt {
				t.Fatalf("expected %d pods, got %d pods", tt.expectedPodsCnt, totalPodCnt)
			}
			if currentRevisionCnt != tt.expectedCurrentRevisionCnt {
				t.Fatalf("expected %d pods with current revision, got %d pods", tt.expectedCurrentRevisionCnt, currentRevisionCnt)
			}
			if updatedRevisionCnt != tt.expectedUpdatedRevisionCnt {
				t.Fatalf("expected %d pods with updated revision, got %d pods", tt.expectedUpdatedRevisionCnt, updatedRevisionCnt)
			}
		})
	}
}

func newFakeControl() *realSync {
	return &realSync{
		Client: fake.NewClientBuilder().Build(),
	}
}

func getPW(replicas int32) *workloadv1alpha1.PartitionWorkload {
	return &workloadv1alpha1.PartitionWorkload{
		TypeMeta: metav1.TypeMeta{
			Kind:       testKind,
			APIVersion: testAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1.NamespaceDefault,
			Name:      testPWName,
			UID:       testUID,
		},
		Spec: workloadv1alpha1.PartitionWorkloadSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test-app",
				},
			},
			Replicas: generalutil.Int32Ptr(replicas),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  testImage,
							Image: testImage,
						},
					},
				},
			},
		},
	}
}

func generatePods(base *v1.Pod, replicas int) []*v1.Pod {
	objs := make([]*v1.Pod, 0, replicas)
	for i := 0; i < replicas; i++ {
		obj := base.DeepCopy()
		obj.Name = fmt.Sprintf("%s-%s-%d", base.Name, base.Spec.Containers[0].Name, i)
		objs = append(objs, obj)
	}
	return objs
}
