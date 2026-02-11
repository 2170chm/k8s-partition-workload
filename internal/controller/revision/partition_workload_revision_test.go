package revision

import (
	"os"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/pkg/controller/history"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
)

func TestMain(m *testing.M) {
	utilruntime.Must(workloadv1alpha1.AddToScheme(scheme.Scheme))
	code := m.Run()
	os.Exit(code)
}

func TestCreateApplyRevision(t *testing.T) {
	r := newFakeControl()
	pw := getPW()
	// pw.Status.CollisionCount = new(int32)
	revision, err := r.NewRevision(pw, 1, pw.Status.CollisionCount)
	if err != nil {
		t.Fatal(err)
	}
	pw.Spec.Template.Spec.Containers[0].Name = "foo"
	if pw.Annotations == nil {
		pw.Annotations = make(map[string]string)
	}
	key := "foo"
	expectedValue := "bar"
	pw.Annotations[key] = expectedValue
	restoredpw, err := r.ApplyRevision(pw, revision)
	if err != nil {
		t.Fatal(err)
	}
	restoredRevision, err := r.NewRevision(restoredpw, 2, restoredpw.Status.CollisionCount)
	if err != nil {
		t.Fatal(err)
	}
	if !history.EqualRevision(revision, restoredRevision) {
		t.Errorf("wanted %v got %v", string(revision.Data.Raw), string(restoredRevision.Data.Raw))
	}
	value, ok := restoredRevision.Annotations[key]
	if !ok {
		t.Errorf("missing annotation %s", key)
	}
	if value != expectedValue {
		t.Errorf("for annotation %s wanted %s got %s", key, expectedValue, value)
	}
}

func TestApplyRevision(t *testing.T) {
	r := newFakeControl()
	pw := getPW()
	currentpw := pw.DeepCopy()
	currentRevision, err := r.NewRevision(pw, 1, pw.Status.CollisionCount)
	if err != nil {
		t.Fatal(err)
	}

	pw.Spec.Template.Spec.Containers[0].Env = []v1.EnvVar{{Name: "foo", Value: "bar"}}
	updatepw := pw.DeepCopy()
	updateRevision, err := r.NewRevision(pw, 2, pw.Status.CollisionCount)
	if err != nil {
		t.Fatal(err)
	}

	restoredCurrentpw, err := r.ApplyRevision(pw, currentRevision)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(currentpw.Spec.Template, restoredCurrentpw.Spec.Template) {
		t.Errorf("want %v got %v", currentpw.Spec.Template, restoredCurrentpw.Spec.Template)
	}

	restoredUpdatepw, err := r.ApplyRevision(pw, updateRevision)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updatepw.Spec.Template, restoredUpdatepw.Spec.Template) {
		t.Errorf("want %v got %v", updatepw.Spec.Template, restoredUpdatepw.Spec.Template)
	}
}

func newFakeControl() *realRevision {
	scheme := runtime.NewScheme()
	utilruntime.Must(workloadv1alpha1.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
	return &realRevision{Scheme: scheme}
}

func getPW() *workloadv1alpha1.PartitionWorkload {
	var collisionCount int32 = 1
	return &workloadv1alpha1.PartitionWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1.NamespaceDefault,
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
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
		},
		Status: workloadv1alpha1.PartitionWorkloadStatus{
			CollisionCount: &collisionCount,
		},
	}
}
