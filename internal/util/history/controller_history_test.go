package history

import (
	"reflect"
	"testing"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/controller/history"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
)

func TestRevisionHistory(t *testing.T) {
	var collisionCount int32 = 10
	var revisionNum int64 = 5
	parent := &workloadv1alpha1.PartitionWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "parent-pw",
			UID:       uuid.NewUUID(),
		},
		Spec: workloadv1alpha1.PartitionWorkloadSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test-app",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test-app",
					},
				},
			},
		},
		Status: workloadv1alpha1.PartitionWorkloadStatus{
			CollisionCount: &collisionCount,
		},
	}

	cr, err := history.NewControllerRevision(parent,
		workloadv1alpha1.GroupVersion.WithKind("PartitionWorkload"),
		parent.Spec.Template.Labels,
		runtime.RawExtension{Raw: []byte(`{}`)},
		revisionNum,
		parent.Status.CollisionCount)
	if err != nil {
		t.Fatalf("Failed to new controller revision: %v", err)
	}

	fakeClient := fake.NewClientBuilder().Build()
	historyControl := NewHistory(fakeClient)

	newCR, err := historyControl.CreateControllerRevision(parent, cr, parent.Status.CollisionCount)
	if err != nil {
		t.Fatalf("Failed to create controller revision: %v", err)
	}

	expectedName := parent.Name + "-" + history.HashControllerRevision(cr, parent.Status.CollisionCount)
	if newCR.Name != expectedName {
		t.Fatalf("Expected ControllerRevision name %v, got %v", expectedName, newCR.Name)
	}

	selector, _ := metav1.LabelSelectorAsSelector(parent.Spec.Selector)
	gotRevisions, err := historyControl.ListControllerRevisions(parent, selector)
	if err != nil {
		t.Fatalf("Failed to list revisions: %v", err)
	}

	expectedRevisions := []*apps.ControllerRevision{newCR}
	if !reflect.DeepEqual(expectedRevisions, gotRevisions) {
		t.Fatalf("List unexpected revisions")
	}

	newCR, err = historyControl.UpdateControllerRevision(newCR, revisionNum+1)
	if err != nil {
		t.Fatalf("Failed to update revision: %v", err)
	}

	if newCR.Revision != revisionNum+1 {
		t.Fatalf("Expected revision %v, got %v", revisionNum+1, newCR.Revision)
	}

	if err = historyControl.DeleteControllerRevision(newCR); err != nil {
		t.Fatalf("Failed to delete revision: %v", err)
	}

	gotRevisions, err = historyControl.ListControllerRevisions(parent, selector)
	if err != nil {
		t.Fatalf("Failed to list revisions: %v", err)
	}
	if len(gotRevisions) > 0 {
		t.Fatalf("Expected no ControllerRevision left, got %v", gotRevisions)
	}
}
