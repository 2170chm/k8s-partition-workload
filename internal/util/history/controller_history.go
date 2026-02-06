package history

import (
	"bytes"
	"context"
	"fmt"

	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	history "k8s.io/kubernetes/pkg/controller/history"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (rh *realHistory) ListControllerRevisions(parent metav1.Object, selector labels.Selector) ([]*apps.ControllerRevision, error) {
	// List all revisions in the namespace that match the selector
	revisions := apps.ControllerRevisionList{}
	err := rh.List(context.TODO(), &revisions, &client.ListOptions{Namespace: parent.GetNamespace(), LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	var owned []*apps.ControllerRevision
	for i := range revisions.Items {
		ref := metav1.GetControllerOf(&revisions.Items[i])
		if ref != nil && ref.UID == parent.GetUID() {
			owned = append(owned, &revisions.Items[i])
		}
	}
	return owned, err
}

func (rh *realHistory) CreateControllerRevision(parent metav1.Object, revision *apps.ControllerRevision, collisionCount *int32) (*apps.ControllerRevision, error) {
	if collisionCount == nil {
		return nil, fmt.Errorf("collisionCount should not be nil")
	}
	ns := parent.GetNamespace()

	// Clone the input
	clone := revision.DeepCopy()

	// Continue to attempt to create the revision updating the name with a new hash on each iteration
	for {
		hash := history.HashControllerRevision(revision, collisionCount)
		// Update the revisions name
		clone.Name = history.ControllerRevisionName(parent.GetName(), hash)

		created := clone.DeepCopy()
		created.Namespace = ns
		err := rh.Create(context.TODO(), created)
		if errors.IsAlreadyExists(err) {
			exists := apps.ControllerRevision{}
			err = rh.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: clone.Name}, &exists)
			if err != nil {
				return nil, err
			}
			if bytes.Equal(exists.Data.Raw, clone.Data.Raw) {
				return &exists, nil
			}
			*collisionCount++
			continue
		}
		return created, err
	}
}

func (rh *realHistory) UpdateControllerRevision(revision *apps.ControllerRevision, newRevision int64) (*apps.ControllerRevision, error) {
	clone := revision.DeepCopy()
	namespacedName := types.NamespacedName{Namespace: clone.Namespace, Name: clone.Name}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if clone.Revision == newRevision {
			return nil
		}
		clone.Revision = newRevision
		updateErr := rh.Update(context.TODO(), clone)
		if updateErr == nil {
			return nil
		}
		got := &apps.ControllerRevision{}
		if err := rh.Get(context.TODO(), namespacedName, got); err == nil {
			clone = got
		}
		return updateErr
	})
	return clone, err
}

func (rh *realHistory) DeleteControllerRevision(revision *apps.ControllerRevision) error {
	return rh.Delete(context.TODO(), revision)
}

// Unused but declared to saitisfy interface
func (rh *realHistory) AdoptControllerRevision(parent metav1.Object, parentKind schema.GroupVersionKind, revision *apps.ControllerRevision) (*apps.ControllerRevision, error) {
	panic("unimplemented")
}

// Unused but declared to saitisfy interface
func (rh *realHistory) ReleaseControllerRevision(parent metav1.Object, revision *apps.ControllerRevision) (*apps.ControllerRevision, error) {
	panic("unimplemented")
}
