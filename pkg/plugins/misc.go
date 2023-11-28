package plugins

import (
	"fmt"
	"strings"

	"sigs.k8s.io/kustomize/api/builtins" //nolint
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/opendatahub-io/opendatahub-operator/v2/components"
)

func transformDeployment(componentName string, patch string, resMap resmap.ResMap) error {
	// set by pkg/plugins/addLabelsplugin.go:ApplyAddLabelsPlugin()
	label := "app.opendatahub.io/" + componentName + "=true"

	plug := builtins.PatchJson6902TransformerPlugin{
		Target: &types.Selector{
			ResId: resid.ResId{
				Gvk: resid.Gvk{Kind: "Deployment"},
			},
			LabelSelector: label,
		},
		JsonOp: patch,
	}

	if err := plug.Transform(resMap); err != nil {
		return err
	}
	return nil
}

func createMultiPatches(values map[string]interface{}) string {
	if len(values) == 0 {
		return ""
	}
	var buf strings.Builder

	buf.WriteString("[")

	for path, value := range values {
		patch := fmt.Sprintf(`{"op": "replace", "path": "%s", "value": "%v"},`, path, value)
		buf.WriteString(patch)
	}

	patch := buf.String()
	// cut last `,` and add final `]`
	patch = patch[:len(patch)-1] + "]"

	return patch
}

func doAdjustReplicas(componentName string, resMap resmap.ResMap, value int) error {
	patch := fmt.Sprintf(`[{"op": "replace", "path": "/spec/replicas", "value": %v}]`, value)
	fmt.Printf("Adjusting replicas, component %s, n %v\n", componentName, value)
	return transformDeployment(componentName, patch, resMap)
}

func AdjustReplicas(componentName string, resMap resmap.ResMap, c components.ComponentInterface) error {
	n := c.GetReplicas()
	if n == nil {
		return nil
	}

	return doAdjustReplicas(componentName, resMap, *n)
}

func UpdateImages(componentName string, resMap resmap.ResMap, c components.ComponentInterface) error {
	patch := createMultiPatches(c.GetPathMap())
	if len(patch) > 0 {
		if err := transformDeployment(componentName, patch, resMap); err != nil {
			return err
		}
	}
	return nil
}
