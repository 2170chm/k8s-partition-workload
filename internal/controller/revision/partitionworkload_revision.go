package revision

import (
	"encoding/json"

	workloadv1alpha1 "github.com/2170chm/k8s-partition-workload/api/v1alpha1"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/controller/history"

	config "github.com/2170chm/k8s-partition-workload/internal/controller/config"
)

func NewRevision(instance *workloadv1alpha1.PartitionWorkload, revision int64, collisionCount *int32) (*apps.ControllerRevision, error) {
	patch, err := getPatch(instance)
	if err != nil {
		return nil, err
	}
	cr, err := history.NewControllerRevision(instance,
		config.ControllerKind,
		instance.Spec.Template.Labels,
		runtime.RawExtension{Raw: patch},
		revision,
		collisionCount)
	if err != nil {
		return nil, err
	}
	if cr.ObjectMeta.Annotations == nil {
		cr.ObjectMeta.Annotations = make(map[string]string)
	}
	for key, value := range instance.Annotations {
		cr.ObjectMeta.Annotations[key] = value
	}
	return cr, nil
}

func getPatch(instance *workloadv1alpha1.PartitionWorkload) ([]byte, error) {
	str, err := runtime.Encode(config.PatchCodec, instance)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(str, &raw)
	objCopy := make(map[string]interface{})
	specCopy := make(map[string]interface{})
	spec := raw["spec"].(map[string]interface{})
	template := spec["template"].(map[string]interface{})

	specCopy["template"] = template
	template["$patch"] = "replace"
	objCopy["spec"] = specCopy
	patch, err := json.Marshal(objCopy)
	return patch, err
}

func ApplyRevision(instance *workloadv1alpha1.PartitionWorkload, revision *apps.ControllerRevision) (*workloadv1alpha1.PartitionWorkload, error) {
	clone := instance.DeepCopy()
	cloneBytes, err := runtime.Encode(config.PatchCodec, clone)
	if err != nil {
		return nil, err
	}
	patched, err := strategicpatch.StrategicMergePatch(cloneBytes, revision.Data.Raw, clone)
	if err != nil {
		return nil, err
	}
	restoredSet := &workloadv1alpha1.PartitionWorkload{}
	if err := json.Unmarshal(patched, restoredSet); err != nil {
		return nil, err
	}
	return restoredSet, nil
}
