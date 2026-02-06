package refmanager

import (
	"reflect"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RefManager provides the method to
type RefManager struct {
	client    client.Client
	selector  labels.Selector
	owner     metav1.Object
	ownerType reflect.Type
	schema    *runtime.Scheme

	once        sync.Once
	canAdoptErr error
}

// New returns a RefManager that exposes
// methods to manage the controllerRef of pods.
func NewRefManager(client client.Client, selector *metav1.LabelSelector, owner metav1.Object, schema *runtime.Scheme) (*RefManager, error) {
	s, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}

	ownerType := reflect.TypeOf(owner)
	if ownerType.Kind() == reflect.Ptr {
		ownerType = ownerType.Elem()
	}
	return &RefManager{
		client:    client,
		selector:  s,
		owner:     owner,
		ownerType: ownerType,
		schema:    schema,
	}, nil
}
