package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	generalutil "github.com/2170chm/k8s-partition-workload/internal/util/general"
)

var _ = Describe("PartitionWorkload Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		var name string
		var typeNamespacedName types.NamespacedName
		var labels map[string]string

		BeforeEach(func() {
			name = fmt.Sprintf("pw-%v", uuid.NewUUID())
			typeNamespacedName = types.NamespacedName{Name: name, Namespace: testNSName}
			labels = map[string]string{"apps": name}
		})
		AfterEach(func() {
			By("Cleanup the specific resource instance PartitionWorkload")

			pw := &workloadv1alpha1.PartitionWorkload{}
			err := k8sClient.Get(ctx, typeNamespacedName, pw)
			if err == nil {
				_ = k8sClient.Delete(ctx, pw)
			} else if !errors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("Cleanup pods")
			pods := &v1.PodList{}
			Expect(k8sClient.List(ctx, pods, client.InNamespace(testNSName), client.MatchingLabels(labels))).To(Succeed())
			for i := range pods.Items {
				_ = k8sClient.Delete(ctx, &pods.Items[i])
			}

			By("Cleanup revisions")
			revisions := &apps.ControllerRevisionList{}
			Expect(k8sClient.List(ctx, revisions, client.InNamespace(testNSName))).To(Succeed())
			for i := range revisions.Items {
				if pw != nil && pw.UID != "" && metav1.IsControlledBy(&revisions.Items[i], pw) {
					_ = k8sClient.Delete(ctx, &revisions.Items[i])
				}
			}
		})

		It(`should successfully reconcile the resource - start with 2 replicas and 1 partition at version 1.
		Update the pod template to version 2 so 1 pod will be updated to version 2. Change partition from 1 to 0.
		Now, the version 2 pod will rollback to version 1. Finally, change partition to 2. Both pods will update to
		version 2 and current(stable) version becomes version 2.`, func() {
			By("Reconciling the created resource")
			partitionworkload := &workloadv1alpha1.PartitionWorkload{}

			By("creating the custom resource for the Kind PartitionWorkload")
			err := k8sClient.Get(ctx, typeNamespacedName, partitionworkload)
			if err != nil && errors.IsNotFound(err) {
				pw := getPW(name, labels, 2, 1, testCurrentImage)
				Expect(k8sClient.Create(ctx, pw)).To(Succeed())
			} else {
				Fail("Previous test PartitionWorkload resource not cleaned up")
			}

			By("validating the initial PartitionWorkload, pods, and revisions")
			Eventually(func(g Gomega) {
				gotPW := &workloadv1alpha1.PartitionWorkload{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, gotPW)).To(Succeed())

				pw_status := gotPW.Status
				g.Expect(pw_status.Conditions).To(BeNil())
				g.Expect(pw_status.Replicas).To(Equal(int32(2)))
				g.Expect(pw_status.UpdatedReplicas).To(Equal(int32(2)))
				g.Expect(pw_status.CurrentRevision).NotTo(BeNil())
				g.Expect(pw_status.UpdateRevision).NotTo(BeNil())
				g.Expect(pw_status.CurrentRevision).To(Equal(pw_status.UpdateRevision))

				pods := &v1.PodList{}
				g.Expect(k8sClient.List(ctx, pods, client.InNamespace(testNSName),
					client.MatchingLabels(labels),
				)).To(Succeed())

				g.Expect(len(pods.Items)).To(Equal(int(2)))

				for i := range pods.Items {
					pod := &pods.Items[i]
					g.Expect(metav1.IsControlledBy(pod, gotPW)).To(BeTrue())
					g.Expect(pod.Spec.Containers[0].Name).To(Equal(testCurrentImage))
				}

				gotRevisions := &apps.ControllerRevisionList{}
				g.Expect(k8sClient.List(ctx, gotRevisions, client.InNamespace(gotPW.Namespace))).To(Succeed())

				revisions := make([]apps.ControllerRevision, 0)
				for _, r := range gotRevisions.Items {
					if metav1.IsControlledBy(&r, gotPW) {
						revisions = append(revisions, r)
					}
				}

				g.Expect(len(revisions)).To(Equal(1))

				var hasCurrent, hasUpdate bool
				for _, r := range revisions {
					if r.Name == gotPW.Status.CurrentRevision {
						hasCurrent = true
					}
					if r.Name == gotPW.Status.UpdateRevision {
						hasUpdate = true
					}
				}
				g.Expect(hasCurrent).To(BeTrue())
				g.Expect(hasUpdate).To(BeTrue())
			}).WithTimeout(timeout).Should(Succeed())

			By("updating the PartitionWorkload spec to version 2")
			pw2 := &workloadv1alpha1.PartitionWorkload{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pw2)).To(Succeed())
			pw2.Spec.Template.Spec.Containers[0].Name = testUpdatedImage
			pw2.Spec.Template.Spec.Containers[0].Image = testUpdatedImage
			Expect(k8sClient.Update(ctx, pw2)).To(Succeed())

			By("Validating the second PartitionWorkload, pods, and revisions")
			Eventually(func(g Gomega) {
				gotPW2 := &workloadv1alpha1.PartitionWorkload{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, gotPW2)).To(Succeed())

				pw2_status := gotPW2.Status
				g.Expect(pw2_status.Conditions).To(BeNil())
				g.Expect(pw2_status.Replicas).To(Equal(int32(2)))
				g.Expect(pw2_status.UpdatedReplicas).To(Equal(int32(1)))
				g.Expect(pw2_status.CurrentRevision).NotTo(BeNil())
				g.Expect(pw2_status.UpdateRevision).NotTo(BeNil())
				g.Expect(pw2_status.CurrentRevision).NotTo(Equal(pw2_status.UpdateRevision))

				pods2 := &v1.PodList{}
				g.Expect(k8sClient.List(ctx, pods2, client.InNamespace(testNSName),
					client.MatchingLabels(labels),
				)).To(Succeed())

				g.Expect(len(pods2.Items)).To(Equal(int(2)))

				newPodCnt := 0
				oldPodCnt := 0
				for i := range pods2.Items {
					pod := &pods2.Items[i]
					g.Expect(metav1.IsControlledBy(pod, gotPW2)).To(BeTrue())
					switch pod.Spec.Containers[0].Name {
					case testUpdatedImage:
						newPodCnt++
					case testCurrentImage:
						oldPodCnt++
					default:
						Fail("A pod with an unknown image was created")
					}
				}
				g.Expect(oldPodCnt).To(Equal(1))
				g.Expect(newPodCnt).To(Equal(1))

				gotRevisions2 := &apps.ControllerRevisionList{}
				g.Expect(k8sClient.List(ctx, gotRevisions2, client.InNamespace(gotPW2.Namespace))).To(Succeed())

				revisions2 := make([]apps.ControllerRevision, 0)
				for _, r := range gotRevisions2.Items {
					if metav1.IsControlledBy(&r, gotPW2) {
						revisions2 = append(revisions2, r)
					}
				}

				g.Expect(len(revisions2)).To(Equal(2))

				var hasCurrent, hasUpdate bool
				for _, r := range revisions2 {
					if r.Name == gotPW2.Status.CurrentRevision {
						hasCurrent = true
					}
					if r.Name == gotPW2.Status.UpdateRevision {
						hasUpdate = true
					}
				}
				g.Expect(hasCurrent).To(BeTrue())
				g.Expect(hasUpdate).To(BeTrue())
			}).WithTimeout(timeout).Should(Succeed())

			By("updating the partition to 0 (roll back all pods to version 1)")
			pw3 := &workloadv1alpha1.PartitionWorkload{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pw3)).To(Succeed())
			pw3.Spec.Partition = generalutil.Int32Ptr(0)
			Expect(k8sClient.Update(ctx, pw3)).To(Succeed())

			By("Validating the third PartitionWorkload, pods, and revisions")
			Eventually(func(g Gomega) {
				gotPW3 := &workloadv1alpha1.PartitionWorkload{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, gotPW3)).To(Succeed())

				pw3_status := gotPW3.Status
				g.Expect(pw3_status.Conditions).To(BeNil())
				g.Expect(pw3_status.Replicas).To(Equal(int32(2)))
				g.Expect(pw3_status.UpdatedReplicas).To(Equal(int32(0)))
				g.Expect(pw3_status.CurrentRevision).NotTo(BeNil())
				g.Expect(pw3_status.UpdateRevision).NotTo(BeNil())
				g.Expect(pw3_status.CurrentRevision).NotTo(Equal(pw3_status.UpdateRevision))

				pods3 := &v1.PodList{}
				g.Expect(k8sClient.List(ctx, pods3, client.InNamespace(testNSName),
					client.MatchingLabels(labels),
				)).To(Succeed())

				g.Expect(len(pods3.Items)).To(Equal(int(2)))

				newPodCnt := 0
				oldPodCnt := 0
				for i := range pods3.Items {
					pod := &pods3.Items[i]
					g.Expect(metav1.IsControlledBy(pod, gotPW3)).To(BeTrue())
					switch pod.Spec.Containers[0].Name {
					case testUpdatedImage:
						newPodCnt++
					case testCurrentImage:
						oldPodCnt++
					default:
						Fail("A pod with an unknown image was created")
					}
				}
				g.Expect(oldPodCnt).To(Equal(2))
				g.Expect(newPodCnt).To(Equal(0))

				gotRevisions3 := &apps.ControllerRevisionList{}
				g.Expect(k8sClient.List(ctx, gotRevisions3, client.InNamespace(gotPW3.Namespace))).To(Succeed())

				revisions3 := make([]apps.ControllerRevision, 0)
				for _, r := range gotRevisions3.Items {
					if metav1.IsControlledBy(&r, gotPW3) {
						revisions3 = append(revisions3, r)
					}
				}

				g.Expect(len(revisions3)).To(Equal(2))

				var hasCurrent, hasUpdate bool
				for _, r := range revisions3 {
					if r.Name == gotPW3.Status.CurrentRevision {
						hasCurrent = true
					}
					if r.Name == gotPW3.Status.UpdateRevision {
						hasUpdate = true
					}
				}
				g.Expect(hasCurrent).To(BeTrue())
				g.Expect(hasUpdate).To(BeTrue())
			}).WithTimeout(timeout).Should(Succeed())

			By("updating the partition to 2 (both pods update to version 2, full rollout)")
			pw4 := &workloadv1alpha1.PartitionWorkload{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pw4)).To(Succeed())
			pw4.Spec.Partition = generalutil.Int32Ptr(2)
			Expect(k8sClient.Update(ctx, pw4)).To(Succeed())

			By("Validating the fourth PartitionWorkload, pods, and revisions")
			Eventually(func(g Gomega) {
				gotPW4 := &workloadv1alpha1.PartitionWorkload{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, gotPW4)).To(Succeed())

				pw4_status := gotPW4.Status
				g.Expect(pw4_status.Conditions).To(BeNil())
				g.Expect(pw4_status.Replicas).To(Equal(int32(2)))
				g.Expect(pw4_status.UpdatedReplicas).To(Equal(int32(2)))
				g.Expect(pw4_status.CurrentRevision).NotTo(BeNil())
				g.Expect(pw4_status.UpdateRevision).NotTo(BeNil())
				g.Expect(pw4_status.CurrentRevision).To(Equal(pw4_status.UpdateRevision))

				pods4 := &v1.PodList{}
				g.Expect(k8sClient.List(ctx, pods4, client.InNamespace(testNSName),
					client.MatchingLabels(labels),
				)).To(Succeed())

				g.Expect(len(pods4.Items)).To(Equal(int(2)))

				newPodCnt := 0
				oldPodCnt := 0
				for i := range pods4.Items {
					pod := &pods4.Items[i]
					g.Expect(metav1.IsControlledBy(pod, gotPW4)).To(BeTrue())
					switch pod.Spec.Containers[0].Name {
					case testUpdatedImage:
						newPodCnt++
					case testCurrentImage:
						oldPodCnt++
					default:
						Fail("A pod with an unknown image was created")
					}
				}
				g.Expect(oldPodCnt).To(Equal(0))
				g.Expect(newPodCnt).To(Equal(2))

				gotRevisions4 := &apps.ControllerRevisionList{}
				g.Expect(k8sClient.List(ctx, gotRevisions4, client.InNamespace(gotPW4.Namespace))).To(Succeed())

				revisions4 := make([]apps.ControllerRevision, 0)
				for _, r := range gotRevisions4.Items {
					if metav1.IsControlledBy(&r, gotPW4) {
						revisions4 = append(revisions4, r)
					}
				}

				g.Expect(len(revisions4)).To(Equal(2))

				var hasCurrent, hasUpdate bool
				for _, r := range revisions4 {
					if r.Name == gotPW4.Status.CurrentRevision {
						hasCurrent = true
					}
					if r.Name == gotPW4.Status.UpdateRevision {
						hasUpdate = true
					}
				}
				g.Expect(hasCurrent).To(BeTrue())
				g.Expect(hasUpdate).To(BeTrue())
			}).WithTimeout(timeout).Should(Succeed())
		})

		It(`should successfully reconcile the resource - start with 1 replicas 1 partition. Scale to 2 replicas.
		Then scale back to 1 replicas`, func() {
			By("Reconciling the created resource")
			partitionworkload := &workloadv1alpha1.PartitionWorkload{}

			By("creating the custom resource for the Kind PartitionWorkload")
			err := k8sClient.Get(ctx, typeNamespacedName, partitionworkload)
			if err != nil && errors.IsNotFound(err) {
				pw := getPW(name, labels, 1, 1, testCurrentImage)
				Expect(k8sClient.Create(ctx, pw)).To(Succeed())
			} else {
				Fail("Previous test PartitionWorkload resource not cleaned up")
			}

			By("validating the initial PartitionWorkload, pods, and revisions")
			Eventually(func(g Gomega) {
				gotPW := &workloadv1alpha1.PartitionWorkload{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, gotPW)).To(Succeed())

				pw_status := gotPW.Status
				g.Expect(pw_status.Conditions).To(BeNil())
				g.Expect(pw_status.Replicas).To(Equal(int32(1)))
				g.Expect(pw_status.UpdatedReplicas).To(Equal(int32(1)))
				g.Expect(pw_status.CurrentRevision).NotTo(BeNil())
				g.Expect(pw_status.UpdateRevision).NotTo(BeNil())
				g.Expect(pw_status.CurrentRevision).To(Equal(pw_status.UpdateRevision))

				pods := &v1.PodList{}
				g.Expect(k8sClient.List(ctx, pods, client.InNamespace(testNSName),
					client.MatchingLabels(labels),
				)).To(Succeed())

				g.Expect(len(pods.Items)).To(Equal(int(1)))

				for i := range pods.Items {
					pod := &pods.Items[i]
					g.Expect(metav1.IsControlledBy(pod, gotPW)).To(BeTrue())
					g.Expect(pod.Spec.Containers[0].Name).To(Equal(testCurrentImage))
				}

				gotRevisions := &apps.ControllerRevisionList{}
				g.Expect(k8sClient.List(ctx, gotRevisions, client.InNamespace(gotPW.Namespace))).To(Succeed())

				revisions := make([]apps.ControllerRevision, 0)
				for _, r := range gotRevisions.Items {
					if metav1.IsControlledBy(&r, gotPW) {
						revisions = append(revisions, r)
					}
				}

				g.Expect(len(revisions)).To(Equal(1))

				var hasCurrent, hasUpdate bool
				for _, r := range revisions {
					if r.Name == gotPW.Status.CurrentRevision {
						hasCurrent = true
					}
					if r.Name == gotPW.Status.UpdateRevision {
						hasUpdate = true
					}
				}
				g.Expect(hasCurrent).To(BeTrue())
				g.Expect(hasUpdate).To(BeTrue())
			}).WithTimeout(timeout).Should(Succeed())

			By("updating the replica to 2")
			pw2 := &workloadv1alpha1.PartitionWorkload{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pw2)).To(Succeed())
			pw2.Spec.Replicas = generalutil.Int32Ptr(2)
			Expect(k8sClient.Update(ctx, pw2)).To(Succeed())

			By("Validating the second PartitionWorkload, pods, and revisions")
			Eventually(func(g Gomega) {
				gotPW2 := &workloadv1alpha1.PartitionWorkload{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, gotPW2)).To(Succeed())

				pw2_status := gotPW2.Status
				g.Expect(pw2_status.Conditions).To(BeNil())
				g.Expect(pw2_status.Replicas).To(Equal(int32(2)))
				g.Expect(pw2_status.UpdatedReplicas).To(Equal(int32(2)))
				g.Expect(pw2_status.CurrentRevision).NotTo(BeNil())
				g.Expect(pw2_status.UpdateRevision).NotTo(BeNil())
				g.Expect(pw2_status.CurrentRevision).To(Equal(pw2_status.UpdateRevision))

				pods2 := &v1.PodList{}
				g.Expect(k8sClient.List(ctx, pods2, client.InNamespace(testNSName),
					client.MatchingLabels(labels),
				)).To(Succeed())

				g.Expect(len(pods2.Items)).To(Equal(int(2)))

				for i := range pods2.Items {
					pod := &pods2.Items[i]
					g.Expect(metav1.IsControlledBy(pod, gotPW2)).To(BeTrue())
					g.Expect(pod.Spec.Containers[0].Name).To(Equal(testCurrentImage))
				}

				gotRevisions2 := &apps.ControllerRevisionList{}
				g.Expect(k8sClient.List(ctx, gotRevisions2, client.InNamespace(gotPW2.Namespace))).To(Succeed())

				revisions2 := make([]apps.ControllerRevision, 0)
				for _, r := range gotRevisions2.Items {
					if metav1.IsControlledBy(&r, gotPW2) {
						revisions2 = append(revisions2, r)
					}
				}

				g.Expect(len(revisions2)).To(Equal(1))

				var hasCurrent, hasUpdate bool
				for _, r := range revisions2 {
					if r.Name == gotPW2.Status.CurrentRevision {
						hasCurrent = true
					}
					if r.Name == gotPW2.Status.UpdateRevision {
						hasUpdate = true
					}
				}
				g.Expect(hasCurrent).To(BeTrue())
				g.Expect(hasUpdate).To(BeTrue())
			}).WithTimeout(timeout).Should(Succeed())

			By("updating the replica to 1")
			pw3 := &workloadv1alpha1.PartitionWorkload{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pw3)).To(Succeed())
			pw3.Spec.Replicas = generalutil.Int32Ptr(1)
			Expect(k8sClient.Update(ctx, pw3)).To(Succeed())

			By("Validating the second PartitionWorkload, pods, and revisions")
			Eventually(func(g Gomega) {
				gotPW3 := &workloadv1alpha1.PartitionWorkload{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, gotPW3)).To(Succeed())

				pw3_status := gotPW3.Status
				g.Expect(pw3_status.Conditions).To(BeNil())
				g.Expect(pw3_status.Replicas).To(Equal(int32(1)))
				g.Expect(pw3_status.UpdatedReplicas).To(Equal(int32(1)))
				g.Expect(pw3_status.CurrentRevision).NotTo(BeNil())
				g.Expect(pw3_status.UpdateRevision).NotTo(BeNil())
				g.Expect(pw3_status.CurrentRevision).To(Equal(pw3_status.UpdateRevision))

				pods3 := &v1.PodList{}
				g.Expect(k8sClient.List(ctx, pods3, client.InNamespace(testNSName),
					client.MatchingLabels(labels),
				)).To(Succeed())

				g.Expect(len(pods3.Items)).To(Equal(int(1)))

				for i := range pods3.Items {
					pod := &pods3.Items[i]
					g.Expect(metav1.IsControlledBy(pod, gotPW3)).To(BeTrue())
					g.Expect(pod.Spec.Containers[0].Name).To(Equal(testCurrentImage))
				}

				gotRevisions3 := &apps.ControllerRevisionList{}
				g.Expect(k8sClient.List(ctx, gotRevisions3, client.InNamespace(gotPW3.Namespace))).To(Succeed())

				revisions3 := make([]apps.ControllerRevision, 0)
				for _, r := range gotRevisions3.Items {
					if metav1.IsControlledBy(&r, gotPW3) {
						revisions3 = append(revisions3, r)
					}
				}

				g.Expect(len(revisions3)).To(Equal(1))

				var hasCurrent, hasUpdate bool
				for _, r := range revisions3 {
					if r.Name == gotPW3.Status.CurrentRevision {
						hasCurrent = true
					}
					if r.Name == gotPW3.Status.UpdateRevision {
						hasUpdate = true
					}
				}
				g.Expect(hasCurrent).To(BeTrue())
				g.Expect(hasUpdate).To(BeTrue())
			}).WithTimeout(timeout).Should(Succeed())
		})
	})
})

func getPW(name string, labels map[string]string, replicas int32, partition int32, image string) *workloadv1alpha1.PartitionWorkload {
	return &workloadv1alpha1.PartitionWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNSName,
			Name:      name,
		},
		Spec: workloadv1alpha1.PartitionWorkloadSpec{
			Replicas: generalutil.Int32Ptr(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  image,
							Image: image,
						},
					},
				},
			},
			Partition: generalutil.Int32Ptr(partition),
		},
	}
}
