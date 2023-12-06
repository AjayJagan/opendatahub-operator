package plugins

import (
	"fmt"
	"strings"

	"sigs.k8s.io/kustomize/api/builtins" //nolint
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"

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

func getEnvArrayPerContainer(c *yaml.RNode) ([]string, error) {
	envKeyStr := make([]string, 0)
	env, err := c.Pipe(yaml.Lookup("env"))
	if err != nil {
		return nil, err
	}

	if env == nil {
		return nil, nil
	}
	for _, e := range env.Content() {
		mapStr, err := yaml.Marshal(e)
		if err != nil {
			return nil, err
		}
		envKeyStr = append(envKeyStr, strings.TrimSpace(strings.Split(strings.Split(string(mapStr), "\n")[0], ":")[1]))
	}
	return envKeyStr, nil
}

func getEnvArray(resMap resmap.ResMap) []string {
	var envArr []string
	for _, r := range resMap.Resources() {
		meta, err := r.GetMeta()
		if err != nil {
			continue
		}

		if meta.TypeMeta.Kind != "Deployment" {
			continue
		}

		containersNode, err := r.Pipe(
			yaml.Lookup("spec", "template", "spec", "containers"))
		if err != nil {
			continue
		}

		containers, err := containersNode.Elements()
		if err != nil {
			continue
		}
		for _, c := range containers {
			arr, err := getEnvArrayPerContainer(c)
			if err != nil {
				fmt.Printf("Error marshalling env in yaml: %v\n", err)
			}
			envArr = append(envArr, arr...)
		}
	}
	return envArr
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
	envArr := getEnvArray(resMap)
	patch := createMultiPatches(c.GetPathMap(envArr))
	if len(patch) > 0 {
		if err := transformDeployment(componentName, patch, resMap); err != nil {
			return err
		}
	}
	return nil
}
