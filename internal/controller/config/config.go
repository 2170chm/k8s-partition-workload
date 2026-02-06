package config

import (
	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	DefaultHistoryLimit = 10
)

var (
	SchemaGroupVersion = workloadv1alpha1.SchemeGroupVersion
	PatchCodec         = scheme.Codecs.LegacyCodec(SchemaGroupVersion)
	ControllerKind     = SchemaGroupVersion.WithKind("PartitionWorkload")
)
