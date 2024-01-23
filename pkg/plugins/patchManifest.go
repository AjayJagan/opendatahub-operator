package plugins

import (
	"fmt"
	"strings"

	"github.com/opendatahub-io/opendatahub-operator/v2/components"
	"sigs.k8s.io/kustomize/api/builtins"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func UpdateImages(resMap resmap.ResMap, c components.ComponentInterface) error {
	envList := getEnvArray(resMap)
	for _, pl := range c.GetLabelAndPathList(envList) {
		var patchStr string
		if pl.Paths != nil {
			patchStr = createMultiPatches(pl.Paths)
		}
		if err := transformDeployment(pl.Label, patchStr, resMap); err != nil {
			return err
		}
	}
	return nil
}

func transformDeployment(label string, patch string, resMap resmap.ResMap) error {
	if patch == "" {
		return nil
	}
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

func createMultiPatches(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	var buf strings.Builder

	buf.WriteString("[")

	for path, value := range values {
		patch := fmt.Sprintf(`{"op": "replace", "path": "%s", "value": "%v"},`, path, value)
		buf.WriteString(patch)
	}

	patchStr := buf.String()
	// cut last `,` and add final `]`
	patchStr = patchStr[:len(patchStr)-1] + "]"

	return patchStr
}

func getEnvArray(resMap resmap.ResMap) []string {
	var envList []string
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
			pc, err := getEnvArrayPerContainer(c)
			if err != nil {
				fmt.Printf("Error marshalling env in yaml: %v\n", err)
			}
			envList = append(envList, pc...)
		}
	}
	return envList
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
		mapByteSlice, err := yaml.Marshal(e)
		if err != nil {
			return nil, err
		}
		mapStr := string(mapByteSlice)
		if strings.HasPrefix(mapStr, "#") {
			mapStr = strings.Join(strings.Split(mapStr, "\n")[1:], "\n")
		}
		envKeyStr = append(envKeyStr, strings.TrimSpace(strings.Split(strings.Split(mapStr, "\n")[0], ":")[1]))
	}
	return envKeyStr, nil
}
