// Package datasciencepipelines provides utility functions to config Data Science Pipelines:
// Pipeline solution for end to end MLOps workflows that support the Kubeflow Pipelines SDK and Tekton
package datasciencepipelines

import (
	"path/filepath"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/components"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
)

var (
	ComponentName = "data-science-pipelines-operator"
	Path          = deploy.DefaultManifestPath + "/" + ComponentName + "/base"
)

// Verifies that Dashboard implements ComponentInterface.
var _ components.ComponentInterface = (*DataSciencePipelines)(nil)

// DataSciencePipelines struct holds the configuration for the DataSciencePipelines component.
// +kubebuilder:object:generate=true
type DataSciencePipelines struct {
	components.Component  `json:""`
	APIServer             components.ControllerImage `json:"apiServer,omitempty"`
	ArtifactManager       components.ControllerImage `json:"artifactManager,omitempty"`
	PersistanceAgent      components.ControllerImage `json:"persistantAgent,omitempty"`
	ScheduledWorkflow     components.ControllerImage `json:"scheduledWorkflow,omitempty"`
	Cache                 components.ControllerImage `json:"cache,omitempty"`
	DSPOperatorController components.ControllerImage `json:"dspo,omitempty"`
}

func (d *DataSciencePipelines) OverrideManifests(_ string) error {
	// If devflags are set, update default manifests path
	if len(d.DevFlags.Manifests) != 0 {
		manifestConfig := d.DevFlags.Manifests[0]
		if err := deploy.DownloadManifests(ComponentName, manifestConfig); err != nil {
			return err
		}
		// If overlay is defined, update paths
		defaultKustomizePath := "base"
		if manifestConfig.SourcePath != "" {
			defaultKustomizePath = manifestConfig.SourcePath
		}
		Path = filepath.Join(deploy.DefaultManifestPath, ComponentName, defaultKustomizePath)
	}

	return nil
}

func (d *DataSciencePipelines) GetComponentName() string {
	return ComponentName
}

func (d *DataSciencePipelines) ReconcileComponent(cli client.Client, owner metav1.Object, dscispec *dsciv1.DSCInitializationSpec, _ bool) error {
	var imageParamMap = map[string]string{
		"IMAGES_APISERVER":         "RELATED_IMAGE_ODH_ML_PIPELINES_API_SERVER_IMAGE",
		"IMAGES_ARTIFACT":          "RELATED_IMAGE_ODH_ML_PIPELINES_ARTIFACT_MANAGER_IMAGE",
		"IMAGES_PERSISTENTAGENT":   "RELATED_IMAGE_ODH_ML_PIPELINES_PERSISTENCEAGENT_IMAGE",
		"IMAGES_SCHEDULEDWORKFLOW": "RELATED_IMAGE_ODH_ML_PIPELINES_SCHEDULEDWORKFLOW_IMAGE",
		"IMAGES_CACHE":             "RELATED_IMAGE_ODH_ML_PIPELINES_CACHE_IMAGE",
		"IMAGES_DSPO":              "RELATED_IMAGE_ODH_DATA_SCIENCE_PIPELINES_OPERATOR_CONTROLLER_IMAGE",
	}
	d.replaceImageValues(imageParamMap)
	enabled := d.GetManagementState() == operatorv1.Managed
	monitoringEnabled := dscispec.Monitoring.ManagementState == operatorv1.Managed

	platform, err := deploy.GetPlatform(cli)
	if err != nil {
		return err
	}
	if enabled {
		// Download manifests and update paths
		if err = d.OverrideManifests(string(platform)); err != nil {
			return err
		}

		// skip check if the dependent operator has beeninstalled, this is done in dashboard

		// Update image parameters only when we do not have customized manifests set
		if dscispec.DevFlags.ManifestsUri == "" && len(d.DevFlags.Manifests) == 0 {
			if err := deploy.ApplyParams(Path, d.SetImageParamsMap(imageParamMap), false); err != nil {
				return err
			}
		}
	}

	if err := deploy.DeployManifestsFromPath(cli, owner, Path, dscispec.ApplicationsNamespace, ComponentName, enabled); err != nil {
		return err
	}
	// CloudService Monitoring handling
	if platform == deploy.ManagedRhods {
		if err := d.UpdatePrometheusConfig(cli, enabled && monitoringEnabled, ComponentName); err != nil {
			return err
		}
		if err = deploy.DeployManifestsFromPath(cli, owner,
			filepath.Join(deploy.DefaultManifestPath, "monitoring", "prometheus", "apps"),
			dscispec.Monitoring.Namespace,
			ComponentName+"prometheus", true); err != nil {
			return err
		}
	}

	return nil
}

func (d *DataSciencePipelines) replaceImageValues(imageParamMap map[string]string) {
	if d.APIServer.Image != "" {
		imageParamMap["IMAGES_APISERVER"] = d.APIServer.Image
	}
	if d.ArtifactManager.Image != "" {
		imageParamMap["IMAGES_ARTIFACT"] = d.ArtifactManager.Image
	}
	if d.PersistanceAgent.Image != "" {
		imageParamMap["IMAGES_PERSISTENTAGENT"] = d.PersistanceAgent.Image
	}
	if d.ScheduledWorkflow.Image != "" {
		imageParamMap["IMAGES_SCHEDULEDWORKFLOW"] = d.ScheduledWorkflow.Image
	}
	if d.Cache.Image != "" {
		imageParamMap["IMAGES_CACHE"] = d.Cache.Image
	}
	if d.DSPOperatorController.Image != "" {
		imageParamMap["IMAGES_DSPO"] = d.DSPOperatorController.Image
	}
}
