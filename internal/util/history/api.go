package history

import (
	history "k8s.io/kubernetes/pkg/controller/history"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewHistory returns an instance of Interface that uses client to communicate with the API Server and lister to list ControllerRevisions.
func NewHistory(c client.Client) history.Interface {
	return &realHistory{Client: c}
}

type realHistory struct {
	client.Client
}
