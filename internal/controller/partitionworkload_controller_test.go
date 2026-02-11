package controller

import (
	"context"
	"reflect"
	"testing"

	"github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/2170chm/k8s-partition-workload/internal/controller/revision"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
	historyutil "github.com/2170chm/k8s-partition-workload/internal/util/history"
)

var (
	testKind         = "PartitionWorkload"
	testAPIVersion   = "workload.scott.dev/v1alpha1"
	testPWName       = "test-pw"
	testPodName      = "test-pod"
	testUID          = types.UID("test")
	revision0        = "0"
	revision1        = "1"
	revision2        = "2"
	testOldImage     = "deprecated_nginx"
	testCurrentImage = "nginx"
	testUpdatedImage = "nginx2"
	productionLabel  = map[string]string{"type": "production"}
	nilLabel         = map[string]string{}
)

func TestClaimPods(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	scheme := runtime.NewScheme()
	workloadv1alpha1.AddToScheme(scheme)
	v1.AddToScheme(scheme)

	pw := newPW(testCurrentImage)

	type test struct {
		name    string
		pods    []*v1.Pod
		claimed []*v1.Pod
	}
	var tests = []test{
		{
			name:    "Controller releases claimed pods when selector doesn't match",
			pods:    []*v1.Pod{newPod("pod1", productionLabel, pw), newPod("pod2", nilLabel, pw)},
			claimed: []*v1.Pod{newPod("pod1", productionLabel, pw)},
		},
		{
			name:    "Claim pods with correct label",
			pods:    []*v1.Pod{newPod("pod3", productionLabel, nil), newPod("pod4", nilLabel, nil)},
			claimed: []*v1.Pod{newPod("pod3", productionLabel, nil)},
		},
	}

	for _, tt := range tests {
		initObjs := []client.Object{pw}
		for i := range tt.pods {
			initObjs = append(initObjs, tt.pods[i])
		}

		reconciler := newFakeControl(scheme, initObjs)

		claimed, err := reconciler.claimPods(pw, scheme, tt.pods)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(reflect.DeepEqual(podToStringSlice(tt.claimed), podToStringSlice(claimed))).To(gomega.BeTrue(),
			"Test case `%s`, claimed wrong pods. Expected %v, got %v", tt.name, podToStringSlice(tt.claimed), podToStringSlice(claimed))
	}
}

func TestGetActiveRevisions(t *testing.T) {
	tests := []struct {
		name     string
		getSetup func(t *testing.T, r *PartitionWorkloadReconciler) (
			pw *workloadv1alpha1.PartitionWorkload,
			revisions []*apps.ControllerRevision,
			expectedCurrentName string,
			expectedUpdatedName string,
			expectedUpdatedRevisionNum int64,
			expectedRevisionCnt int,
			expectCurrentEqualsUpdated bool,
		)
	}{
		{
			name: "new revision -> current revision keeps old number, new revision has new number",
			getSetup: func(t *testing.T, r *PartitionWorkloadReconciler) (*workloadv1alpha1.PartitionWorkload, []*apps.ControllerRevision, string, string, int64, int, bool) {
				currentPW := newPW(testCurrentImage)
				updatedPW := newPW(testUpdatedImage)

				existing, err := r.RevisionControl.NewRevision(currentPW, 1, generalutil.Int32Ptr(0))
				if err != nil {
					t.Errorf("Failed to create revision. err: %v", err)
				}
				err = r.Client.Create(context.TODO(), existing)
				if err != nil {
					t.Errorf("Failed to commit revision to client. err: %v", err)
				}
				updatedPW.Status.CurrentRevision = existing.Name
				updatedPW.Status.UpdateRevision = existing.Name

				return updatedPW, []*apps.ControllerRevision{existing}, existing.Name, "", 2, 2, false
			},
		},
		{
			name: "no change -> reuse previous revision",
			getSetup: func(t *testing.T, r *PartitionWorkloadReconciler) (*workloadv1alpha1.PartitionWorkload, []*apps.ControllerRevision, string, string, int64, int, bool) {
				pw := newPW(testCurrentImage)

				existing, err := r.RevisionControl.NewRevision(pw, 1, generalutil.Int32Ptr(0))
				if err != nil {
					t.Errorf("Failed to create revision. err: %v", err)
				}
				err = r.Client.Create(context.TODO(), existing)
				if err != nil {
					t.Errorf("Failed to commit revision to client. err: %v", err)
				}
				pw.Status.CurrentRevision = existing.Name
				pw.Status.UpdateRevision = existing.Name

				return pw, []*apps.ControllerRevision{existing}, existing.Name, existing.Name, 1, 1, false
			},
		},
		{
			name: "no change from a revision not immediately before -> reuse previous revision and bump revision num",
			getSetup: func(t *testing.T, r *PartitionWorkloadReconciler) (*workloadv1alpha1.PartitionWorkload, []*apps.ControllerRevision, string, string, int64, int, bool) {
				pw1 := newPW(testOldImage)
				pw2 := newPW(testCurrentImage)

				existing1, err := r.RevisionControl.NewRevision(pw1, 1, generalutil.Int32Ptr(0))
				if err != nil {
					t.Errorf("Failed to create revision. err: %v", err)
				}
				err = r.Client.Create(context.TODO(), existing1)
				if err != nil {
					t.Errorf("Failed to commit revision to client. err: %v", err)
				}
				existing2, err := r.RevisionControl.NewRevision(pw2, 2, generalutil.Int32Ptr(0))
				if err != nil {
					t.Errorf("Failed to create revision. err: %v", err)
				}
				err = r.Client.Create(context.TODO(), existing2)
				if err != nil {
					t.Errorf("Failed to commit revision to client. err: %v", err)
				}

				pw1.Status.CurrentRevision = existing2.Name
				pw1.Status.UpdateRevision = existing2.Name

				return pw1, []*apps.ControllerRevision{existing1, existing2}, existing2.Name, existing1.Name, 3, 2, false
			},
		},
		{
			name: "first revision",
			getSetup: func(t *testing.T, r *PartitionWorkloadReconciler) (*workloadv1alpha1.PartitionWorkload, []*apps.ControllerRevision, string, string, int64, int, bool) {
				pw := newPW(testUpdatedImage)

				pw.Status.CurrentRevision = ""
				pw.Status.UpdateRevision = ""

				return pw, []*apps.ControllerRevision{}, "", "", 1, 1, true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(workloadv1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
			g.Expect(v1.AddToScheme(scheme)).To(gomega.Succeed())
			g.Expect(apps.AddToScheme(scheme)).To(gomega.Succeed())

			r := newFakeControl(scheme, nil)

			pw, revisions, expectedCurrentName, expectedUpdatedName, expectedUpdatedRevisionNum, expectedRevisionCnt, expectCurrentEqualsUpdated := tt.getSetup(t, r)

			currentRevision, updatedRevision, _, err := r.getActiveRevisions(pw, revisions)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			if expectCurrentEqualsUpdated {
				g.Expect(currentRevision.Name).To(gomega.Equal(updatedRevision.Name))
			} else {
				g.Expect(currentRevision.Name).To(gomega.Equal(expectedCurrentName))
			}

			g.Expect(updatedRevision.Revision).To(gomega.Equal(expectedUpdatedRevisionNum))

			if expectedUpdatedName == "" {
				g.Expect(updatedRevision.Name).NotTo(gomega.Equal(expectedCurrentName))
			} else {
				g.Expect(updatedRevision.Name).To(gomega.Equal(expectedUpdatedName))
			}

			var list apps.ControllerRevisionList
			g.Expect(r.Client.List(context.TODO(), &list)).To(gomega.Succeed())
			g.Expect(list.Items).To(gomega.HaveLen(expectedRevisionCnt))
		})
	}
}

func newFakeControl(scheme *runtime.Scheme, initObjs []client.Object) *PartitionWorkloadReconciler {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
	return &PartitionWorkloadReconciler{
		Client:          fakeClient,
		Scheme:          scheme,
		HistoryControl:  historyutil.NewHistory(fakeClient),
		RevisionControl: revision.NewRevisionControl(fakeClient, scheme),
	}
}

func newPod(podName string, label map[string]string, owner metav1.Object) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Labels:    label,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test",
					Image: "foo/bar",
				},
			},
		},
	}
	if owner != nil {
		pod.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(owner, apps.SchemeGroupVersion.WithKind("Fake"))}
	}
	return pod
}

func newPW(image string) *workloadv1alpha1.PartitionWorkload {
	return &workloadv1alpha1.PartitionWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1.NamespaceDefault,
			Name:      testPWName,
			UID:       testUID,
		},
		Spec: workloadv1alpha1.PartitionWorkloadSpec{
			Replicas: generalutil.Int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: productionLabel},
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  image,
							Image: image,
						},
					},
				},
			},
		},
	}
}

func podToStringSlice(pods []*v1.Pod) []string {
	var names []string
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	return names
}

// var _ = Describe("PartitionWorkload Controller", func() {
// 	Context("When reconciling a resource", func() {
// 		const resourceName = "test-resource"

// 		ctx := context.Background()

// 		typeNamespacedName := types.NamespacedName{
// 			Name:      resourceName,
// 			Namespace: "default", // TODO(user):Modify as needed
// 		}
// 		partitionworkload := &workloadv1alpha1.PartitionWorkload{}

// 		BeforeEach(func() {
// 			By("creating the custom resource for the Kind PartitionWorkload")
// 			err := k8sClient.Get(ctx, typeNamespacedName, partitionworkload)
// 			if err != nil && errors.IsNotFound(err) {
// 				resource := &workloadv1alpha1.PartitionWorkload{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      resourceName,
// 						Namespace: "default",
// 					},
// 					// TODO(user): Specify other spec details if needed.
// 				}
// 				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
// 			}
// 		})

// 		AfterEach(func() {
// 			// TODO(user): Cleanup logic after each test, like removing the resource instance.
// 			resource := &workloadv1alpha1.PartitionWorkload{}
// 			err := k8sClient.Get(ctx, typeNamespacedName, resource)
// 			Expect(err).NotTo(HaveOccurred())

// 			By("Cleanup the specific resource instance PartitionWorkload")
// 			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
// 		})
// 		It("should successfully reconcile the resource", func() {
// 			By("Reconciling the created resource")
// 			controllerReconciler := &PartitionWorkloadReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}

// 			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: typeNamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
// 			// Example: If you expect a certain status condition after reconciliation, verify it here.
// 		})
// 	})
// })
