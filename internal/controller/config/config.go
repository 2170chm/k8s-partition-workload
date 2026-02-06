package config

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	// Note that the historySize is the max number of non-live revisions allowed
	// A live revision is a revision that is either being used by at least one
	// pod or is the updaterevision or the currenrevision of PartitionWorkload
	// It does not represent the total number of controllerrevisions
	DefaultHistoryLimit = 10
)

var (
	SchemaGroupVersion = workloadv1alpha1.SchemeGroupVersion
	PatchCodec         = scheme.Codecs.LegacyCodec(SchemaGroupVersion)
	ControllerKind     = SchemaGroupVersion.WithKind("PartitionWorkload")
)
