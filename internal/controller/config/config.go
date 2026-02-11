package config

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	Scheme             = scheme.Scheme
	SchemaGroupVersion = workloadv1alpha1.SchemeGroupVersion
	PatchCodec         = scheme.Codecs.LegacyCodec(SchemaGroupVersion)
)

func init() {
	utilruntime.Must(v1.AddToScheme(Scheme))
	utilruntime.Must(apps.AddToScheme(Scheme))
	utilruntime.Must(workloadv1alpha1.AddToScheme(Scheme))
}
