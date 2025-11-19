# Component Migration Guide: RHOAI 2.25.2 → 3.3.0

This document provides comprehensive migration steps for all RHOAI components when upgrading from version 2.25.2 to 3.3.0. This guide complements the [Upgrade Troubleshooting Guide](UPGRADE-TROUBLESHOOTING-2.25.2-to-3.3.0.md).

**Document Purpose:**
- Detailed before/during/after upgrade procedures for each component
- Component-specific migration steps and validation
- Complete upgrade workflow understanding
- Reference for future upgrades

## Table of Contents

1. [Component Migration Order](#component-migration-order)
2. [Before Upgrade](#before-upgrade)
   - [Prerequisites by Component](#prerequisites-by-component)
   - [Preparation Steps](#preparation-steps)
3. [During Upgrade](#during-upgrade)
4. [After Upgrade](#after-upgrade)
5. [Verification Procedures](#verification-procedures)

---

## Component Migration Order

**NOTE:** Please treat individual component sections as canonical. This aggregation provides a defined order and may require updates when component-specific instructions change.

**Last Updated:** 2026-02-10 (copied from source at 2026-02-09 17:00 EST)

---

## Before Upgrade

### Prerequisites by Component

#### Kueue

**Requirements:**
- DSC resource must have condition `status.conditions.Type=Ready` with `Status=True`
- Kueue component must be in `Removed` or `Unmanaged` state (DSC field `spec.components.kueue.managementState`)

**If Kueue is set to Managed:**

A migration to RHBoK (Red Hat build of Kueue) is required following:
- Documentation: [Managing workloads with Kueue - Migrating to RHBoK](https://docs.redhat.com/en/documentation/red_hat_openshift_ai_self-managed/2.25/html/managing_openshift_ai/managing-workloads-with-kueue#migrating-to-the-rhbok-operator_kueue)

**IMPORTANT Framework Configuration:**

If you are relying on the default RHOAI Kueue configuration (i.e., you have never modified the `kueue-manager-config` ConfigMap in `<applications-namespace>`), and you want to ensure you keep the same set of enabled frameworks:

```bash
oc annotate configmap kueue-manager-config -n <applications-namespace> opendatahub.io/managed=false
```

Otherwise the enabled frameworks will change from:
```
"batch/job"
"kubeflow.org/mpijob"
"ray.io/rayjob"
"ray.io/raycluster"
"jobset.x-k8s.io/jobset"
"kubeflow.org/paddlejob"
"kubeflow.org/pytorchjob"
"kubeflow.org/tfjob"
"kubeflow.org/xgboostjob"
"workload.codeflare.dev/appwrapper"
```

To:
```
Deployment
Pod
PyTorchJob
RayCluster
RayJob
StatefulSet
```

**CRITICAL:** Before proceeding with the upgrade, the `spec.components.kueue.managementState` field in the DSC resource must first be set to `Removed` and then to `Unmanaged`, as described in the migration steps document. The `Managed` value for the Kueue component is **not supported** in 3.0.

---

#### AI Core Platform

**Prerequisites:**
- Upgrading to OCP 4.19 or 4.20 is complete and the cluster is in a stable state
- RHOAI 2.25 is installed, its CSV status is "Succeeded"
- DSCI and DSC are "Ready"
- InstallPlan of RHOAI 2.25 is set to Manual (not required but prevents accidental automatic upgrade)
- If Kueue is set to Managed, either set it as Removed or migrate Kueue to Unmanaged
  - Reference: [Customer RHOAI 2.25 to 3.x Upgrade Guide](https://docs.redhat.com/en/documentation/red_hat_openshift_ai_self-managed/)
- If OSSM 2 is installed and in use for KServe, complete the Model Serving & Metrics pre-upgrade steps to remove OSSM 2

---

#### Model Serving

**Prerequisites:**

1. **OpenShift Version:** Access to an OpenShift cluster running version 4.19.9 or later

2. **Serving Runtime:** Ensure no InferenceServices rely on Serving Runtimes that will be deprecated in RHOAI 3
   - Refer to Model Serving Runtimes component migration steps
   - Deprecated runtimes: Caikit-standalone, Caikit-TGIS, TGIS API of vLLM

3. **Kueue Migration:** If using Kueue with InferenceServices, migrate to standalone Red Hat build of Kueue Operator
   - Reference: [Migrating to the Red Hat build of Kueue Operator](https://docs.redhat.com/en/documentation/red_hat_openshift_ai_self-managed/2.25/html/managing_openshift_ai/managing-workloads-with-kueue#migrating-to-the-rhbok-operator_kueue)

4. **Service Mesh Operator:**
   - ⚠️ **CRITICAL:** Administrators must audit the cluster for any non-RHOAI applications relying on OpenShift Service Mesh v2
   - Consult with application owners to migrate these workloads before initiating the RHOAI 3.x upgrade
   - **If you cannot remove OSSM v2 due to non-RHOAI dependencies, you SHOULD NOT upgrade to RHOAI 3.x**

**Service Mesh Conflict Warning:**

If a conflicting OSSM v2.x subscription is present when you create a GatewayClass resource:
- Cluster Ingress Operator attempts to install required OSSM v3.x components
- This installation operation **fails**
- Gateway API resources (Gateway, HTTPRoute) have no effect
- No proxy gets configured to route traffic
- In OpenShift 4.19: This failure is **silent**
- In OpenShift 4.20+: This conflict causes the ingress ClusterOperator to report **Degraded** status

Reference: [Getting started with Gateway API for the Ingress Operator](https://docs.redhat.com/en/documentation/openshift_container_platform/)

**Remediation:**
- Migrate external dependencies to Service Mesh v3
- Reference: [Migrating from Service Mesh 2 to Service Mesh 3](https://docs.redhat.com/en/documentation/openshift_container_platform/)

---

#### Model Serving Runtimes

**Prerequisites:**

1. **OpenShift Version:** Access to an OpenShift cluster running version 4.19.9 or later

2. **Serving Runtime Compatibility:**
   - Ensure no InferenceServices rely on Serving Runtimes removed in RHOAI 3.0 onwards:
     - OpenVINO Model Server – Multi-model serving
     - Caikit-TGIS
     - Caikit-Standalone Runtime

3. **vLLM TGIS API Removed:**
   - TGIS Adapters are removed as of RHOAI 3.0
   - TGIS API endpoints are incompatible
   - **If you use TGIS API, remain on version 2.25** until you can migrate to OpenAI-compatible API endpoints provided by vLLM
   - These endpoints are the default for many frameworks: LangChain, LlamaIndex, and others

4. **vLLM V0 Engine Removed:**
   - Starting with RHOAI 3.0 (RHAIIS 3.2.3), the V0 engine has been completely removed
   - The `VLLM_USE_V1=0` option is no longer supported
   - Refer to the vLLM/RHAIIS Component guide to check if your deployed models use this setting in RHOAI 2.25

5. **Model Serving Workloads:**
   - Refer to the Model Serving Component migration steps for workload migration

---

#### Workbenches / Notebooks Server

**Prerequisites:**

1. **Cluster Administrator Role:** Required for performing upgrades, along with access to:
   - System with Bash and `oc` CLI for performing updates
   - OpenShift AI operator should be upgraded to latest version of 2.25.2

2. **Workbench Preparation:**
   - Upgrade all user workbenches to the latest version tag of the selected image for ease of migration
   - If using custom notebook images: ⚠️ **Requires modification** to work with new authentication method
   - If using OpenShift AI provided RStudio or Code Server: Upgrade to latest tag (`2025.2`)

---

#### Ray Training Operator

**Prerequisites:**

These steps require **cluster-admin level permissions**. Execute from a workstation with:
- `oc` CLI matching the cluster's OpenShift version
- Python 3.6+ available as `python3` executable
- Ability to execute bash commands
- Ray upgrade script (`ray_cluster_migration.py`) stored in a directory

**Important Notes:**
- These steps should be conducted in tandem with:
  - OpenShift AI admin
  - Users who have created the Ray clusters to be migrated
- Parts of the migration might cause temporary downtime
- Users should be warned ahead of time

**Migration Tool Overview:**

When upgrading from RHOAI 2.25 to RHOAI 3.3, existing RayClusters need migration to work with the new architecture. This migration tool helps you:

1. Back up your RayCluster configurations before the upgrade
2. Verify your cluster is ready for the upgrade (pre-flight checks)
3. Migrate your RayClusters after the upgrade is complete

**Tool Features:**
- **Staged approach:** Test on a single cluster before migrating everything
- **Idempotent:** Safe to run multiple times
- **Non-destructive:** RayCluster CR backups are created, nothing is deleted automatically

**Steps for KubeRay:**

1.1. **Confirm DataScienceCluster name is "default":**

```bash
oc get datasciencecluster
# Expected output:
# NAME         READY   REASON
# default-dsc  True
```

1.2. **Mark codeflare-operator as Removed in the default DSC:**

```bash
oc patch datasciencecluster default-dsc --type merge -p '{"spec":{"components":{"codeflare":{"managementState":"Removed"}}}}'
```

1.3. **Install cert-manager (if not already in the cluster):**

- Go to OperatorHub in the OpenShift console
- Search for "cert-manager"
- Install the cert-manager operator (if it's not already there)
- Wait for it to be ready

**Steps for Migration Script:**

2.1. **Python Environment:**

Verify Python 3.6 or later is installed:

```bash
python3 --version
```

2.2. **Install Required Packages:**

```bash
pip3 install kubernetes PyYAML
```

2.3. **Required Permissions:**

The migration tool requires different permissions for each step. Run these checks to verify access:

```bash
echo "=== Pre-Upgrade Permissions ==="
oc auth can-i list namespaces && echo "  [OK] list namespaces" || echo "  [FAIL] list namespaces"
oc auth can-i list rayclusters.ray.io --all-namespaces && echo "  [OK] list rayclusters" || echo "  [FAIL] list rayclusters"
oc auth can-i get rayclusters.ray.io --all-namespaces && echo "  [OK] get rayclusters" || echo "  [FAIL] get rayclusters"
oc auth can-i get customresourcedefinitions.apiextensions.k8s.io && echo "  [OK] get CRDs" || echo "  [FAIL] get CRDs"

echo ""
echo "=== Post-Upgrade Permissions ==="
oc auth can-i update rayclusters.ray.io --all-namespaces && echo "  [OK] update rayclusters" || echo "  [FAIL] update rayclusters"
oc auth can-i list httproutes.gateway.networking.k8s.io --all-namespaces && echo "  [OK] list httproutes" || echo "  [FAIL] list httproutes"
oc auth can-i get httproutes.gateway.networking.k8s.io --all-namespaces && echo "  [OK] get httproutes" || echo "  [FAIL] get httproutes"
```

---

#### TrustyAI

**Prerequisites:**

- OpenShift Cluster with Red Hat OpenShift AI (RHOAI) installed via Data Science Cluster (DSC) custom resource
- DSC must have TrustyAI set to `Managed`
- `oc` and cluster admin access
- A LLM deployed on vLLM ServingRuntime in your working namespace
- A ConfigMap with GuardrailsOrchestrator configuration
- A GuardrailsOrchestrator instance

---

#### LlamaStack

**Prerequisites:**

Before starting the upgrade, ensure you have:

1. **Access Requirements:**
   - Application source code or notebooks that integrate with Llama Stack
   - Cluster administrator access to inspect InferenceService and RBAC configuration

2. **Model Deployment:**
   - A LLM deployed on vLLM ServingRuntime in your working namespace
   - Ensure you have an OpenAI compatible LLM deployed on vLLM ServingRuntime
   - This could be an InferenceService (which should be migrated to RawDeployment) or an alternative external service

3. **PostgreSQL Database (Required in 3.3):**
   - RHOAI 3.3 requires PostgreSQL for storage backend (replaces SQLite)
   - Deploy a PostgreSQL instance and create a database for LlamaStack

4. **Embedding Model Endpoint (Required in 3.3):**
   - RHOAI 3.3 requires a separate embedding model endpoint
   - Ensure you have:
     - A deployed embedding model (e.g., `ibm-granite/granite-embedding-125m-english`)
     - The endpoint URL and API token

5. **⚠️ Backup Warning:**
   - The upgrade to PostgreSQL backend means **existing SQLite data will not be automatically migrated**
   - This includes:
     - Agent state
     - Vector database metadata
     - Telemetry data
     - File metadata
   - **Plan accordingly if you need to preserve this data**

**NOTE:** The Llama Stack distribution for an instance can be pinned, allowing a user to upgrade the Llama Stack Operator independently before updating the workload.

This pinning involves replacing "rh-dev" with the correct distribution image. In this example, if a customer wishes to remain on the llama-stack version distributed with RHOAI 2.25 and continue using `0.2.22.2+rhai0`:

```yaml
spec:
  server:
    distribution:
      image: registry.redhat.io/rhoai/odh-llama-stack-core-rhel9:rhoai-2.25
```

---

#### vLLM/RHAIIS

**Prerequisites:**

- You have models deployed on the single-model serving platform using the vLLM serving runtime in Red Hat OpenShift AI 2.25
- You have access to the OpenShift AI dashboard or the OpenShift CLI (`oc`)

---

### Preparation Steps

Provide steps to indicate the order in which components need to be prepared for upgrade.

#### AI Pipelines

**No action required** unless you are using GitOps or custom scripts to upgrade RHOAI, in which case:

1. **Patch the DSC:** Rename "dataSciencePipelines" component name to "aiPipelines" in the spec

2. **Update Custom Roles:** If you have users or service accounts with custom roles for AI Pipelines (formerly Data Science Pipelines), update those roles to include the `datasciencepipelinesapplications/api` subresource with the corresponding verb matching the expected permission.

**Example command to patch your role:**

```bash
ROLE_NAME=your-role-name
NAMESPACE=your-namespace
DSPA_NAME=your-dspa-name

kubectl patch role "$ROLE_NAME" -n "$NAMESPACE" --type=json -p "[
  {
    \"op\": \"add\",
    \"path\": \"/rules/-\",
    \"value\": {
      \"apiGroups\": [\"datasciencepipelinesapplications.opendatahub.io\"],
      \"resources\": [\"datasciencepipelinesapplications/api\"],
      \"resourceNames\": [\"$DSPA_NAME\"],
      \"verbs\": [\"get\",\"create\",\"update\",\"patch\",\"delete\"]
    }
  }
]"
```

3. **Update Manifests:** If you use GitOps or custom scripts for deploying AI Pipelines, update the manifest to use apiVersion v1:

```yaml
apiVersion: datasciencepipelinesapplications.opendatahub.io/v1
kind: DataSciencePipelinesApplication
metadata:
  name: your-dspa-name
  namespace: your-namespace
```

Or fetch an existing one which should give you the manifest with the right API version:

```bash
oc get dspa $DSPA_NAME -n $NAMESPACE -o yaml
```

---

#### AI Core Platform

**Component Workload Migration:**

Any component workloads that need to be migrated before upgrade should be migrated at this point (e.g., ModelMesh and Serverless InferenceServices).

**⚠️ STEPS SHARED WITH MODEL SERVING & METRICS FOR BETTER CONTEXT:**

1. **Update the RHOAI 2.25.2 DSC** as listed below. One at a time, ensure DSC returns to ready state:

```yaml
kserve:
  defaultDeploymentMode: RawDeployment   # <<< This needs to change
  managementState: Managed
  nim:
    managementState: Managed
  rawDeploymentServiceConfig: Headless
  serving:
    ingressGateway:
      certificate:
        type: OpenshiftDefaultIngress
    managementState: Removed        # <<< This needs to change if Managed on 2.25, if Unmanaged, leave as is
    name: knative-serving

kueue:
  defaultClusterQueueName: default
  defaultLocalQueueName: default
  managementState: Removed/Unmanaged    # <<< This needs to change, depending on the config, see prerequisites

codeflare:
  managementState: Removed    # <<< This needs to change, shared with Ray Training Operator

modelmeshserving:
  managementState: Removed    # <<<< This needs to change
```

2. **Update the DSCI** as listed below to disable serviceMesh:

```yaml
spec:
  applicationsNamespace: redhat-ods-applications
  monitoring:
    managementState: Managed
    metrics: {}
    namespace: redhat-ods-monitoring
  serviceMesh:
    auth:
      audiences:
        - 'https://kubernetes.default.svc'
    controlPlane:
      metricsCollection: Istio
      name: data-science-smcp
      namespace: istio-system
    managementState: Removed  # <<<< This needs to change
  trustedCABundle:
    customCABundle: ''
    managementState: Managed
```

3. **Remove the following operators including operands:**
   - serverless
   - servicemesh 2
   - Authorino

4. **Install the following Operators** if you are going to use llm-d (LLMInferenceServices):
   - Red Hat Connectivity Link (this will reinstall Authorino)
   - cert-manager Operator for Red Hat OpenShift (can be already installed if Kueue is Unmanaged)
   - Red Hat build of Leader Worker Set

5. **Configure operators installed** by creating these CRs:
   - Kuadrant CR
   - Authorino configuration
   - LWS CR
   - KServe gateway/gatewayclass

**⚠️ END OF SHARED STEPS**

6. **Optionally Install Job Set Operator** if you are going to use Trainer v2 after the upgrade

7. **Any other configuration** of dependent operators should be done at this point

8. **Update the subscription for RHOAI** to 'stable-3.x' OR 'stable-3.3'. Ensure Update approval is still set to 'Manual'

9. **Approve InstallPlan** - upgrade starts

---

#### Model Serving

1. **Create backups** of your custom `inferenceservice-config` ConfigMap

2. **Convert ModelMesh and Serverless Deployment Mode to RawDeployment:**
   - Follow the Knowledge Base article: [Converting ModelMesh and Serverless InferenceServices to RawDeployment (Standard) Mode](https://access.redhat.com/articles/)
   - **Update model endpoints and tokens** in your downstream applications

3. **Update Data Science Cluster (DSC) Configuration:**
   - Modify the configuration to use RawDeployment for KServe
   - Switch the serving components' management state to Removed

Required YAML changes:

```yaml
spec:
  kserve:
    defaultDeploymentMode: RawDeployment
    managementState: Managed
    # ...
  serving:
    # ...
    managementState: Removed
```

4. **Update DSC Initialization (DSCI):**

Update the DSCI configuration to remove Service Mesh management:

```yaml
spec:
  serviceMesh:
    managementState: Removed
```

5. **Uninstall Legacy Operators:**

Once all workloads are migrated to RawDeployment, uninstall the following operators and delete their namespaces:
- Serverless Operator
- Service Mesh v2 Operator
- Authorino Operator

6. **Install Operators for Distributed Inference (llm-d workloads):**

If you have Distributed Inference (llm-d) workloads, install:
- **cert-manager operator**
  - Reference: [cert-manager Operator For Red Hat OpenShift](https://docs.redhat.com/en/documentation/cert-manager_operator_for_red_hat_openshift/)
- **Red Hat Connectivity Link**
  - Reference: [Installing Connectivity Link on OpenShift](https://docs.redhat.com/en/documentation/red_hat_connectivity_link/)
  - **Note:** Authorino will still be used in RHOAI 3, but through RHCL instead of standalone
  - **For KServe RawDeployment:** RHCL is not needed

7. **Configure RHCL:**

Configure RHCL according to official docs: [Configuring authentication for Distributed Inference with llm-d using Red Hat Connectivity Link](https://docs.redhat.com/en/documentation/red_hat_openshift_ai_self-managed/3.3/html/serving_models/serving-large-models_serving-large-models#configuring-authentication-for-distributed-inference_serving-large-models)

**⚠️ Breaking Change: Authentication Enabled by Default**

In 3.0+, LLMInferenceService will have authentication and authorization enabled by default. To prevent service disruptions:

**Option 1: Disable authentication (temporary workaround):**

Annotate the LLMInferenceService:

```yaml
metadata:
  annotations:
    security.opendatahub.io/enable-auth: 'false'
```

**Option 2: Update clients to use authentication (recommended):**

Update clients to send requests including `Authorization: "Bearer <token>"`, where the token is a user or ServiceAccount token with the `get` permissions on the specific LLMInferenceService resource.

Example RBAC setup:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-llmisvc-sa
  namespace: my-llm-project
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-llmisvc-role
  namespace: my-llm-project
rules:
- apiGroups:
  - "serving.kserve.io"
  resources:
  - llminferenceservices
  resourceNames:
  - my-llm
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: my-llmisvc-rolebinding
  namespace: my-llm-project
subjects:
  - kind: ServiceAccount
    name: my-llmisvc-sa
    namespace: my-llm-project
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: my-llmisvc-role
```

**⚠️ Breaking Change: Scheduler Arguments**

To prevent service disruptions caused by breaking changes in scheduler arguments and subsequent rollouts, LLMInferenceService resources can be specifically annotated to utilize particular LLMInferenceServiceConfig instances as base configurations:

```yaml
apiVersion: serving.kserve.io
kind: LLMInferenceService
metadata:
  annotations:
    serving.kserve.io/config-llm-template: kserve-config-llm-template
    serving.kserve.io/config-llm-decode-template: kserve-config-llm-decode-template
    serving.kserve.io/config-llm-worker-data-parallel: kserve-config-llm-worker-data-parallel
    serving.kserve.io/config-llm-decode-worker-data-parallel: kserve-config-llm-decode-worker-data-parallel
    serving.kserve.io/config-llm-prefill-template: kserve-config-llm-prefill-template
    serving.kserve.io/config-llm-prefill-worker-data-parallel: kserve-config-llm-prefill-worker-data-parallel
    serving.kserve.io/config-llm-scheduler: kserve-config-llm-scheduler
    serving.kserve.io/config-llm-router-route: kserve-config-llm-router-route
```

Or using kubectl patch:

```bash
kubectl patch llmisvc my-llm -n my-llm-project \
  --subresource=status \
  --type=merge \
  -p '{
    "status": {
      "annotations": {
        "serving.kserve.io/config-llm-template": "kserve-config-llm-template",
        "serving.kserve.io/config-llm-decode-template": "kserve-config-llm-decode-template",
        "serving.kserve.io/config-llm-worker-data-parallel": "kserve-config-llm-worker-data-parallel",
        "serving.kserve.io/config-llm-decode-worker-data-parallel": "kserve-config-llm-decode-worker-data-parallel",
        "serving.kserve.io/config-llm-prefill-template": "kserve-config-llm-prefill-template",
        "serving.kserve.io/config-llm-prefill-worker-data-parallel": "kserve-config-llm-prefill-worker-data-parallel",
        "serving.kserve.io/config-llm-scheduler": "kserve-config-llm-scheduler",
        "serving.kserve.io/config-llm-router-route": "kserve-config-llm-router-route"
      }
    }
  }'
```

**Note:** For OpenShift AI 3.2 and later, the base configurations for LLMInferenceServiceConfig are prefixed with the version number (e.g., `v3.3.0-config-llm-template`).

**Scheduler Argument Changes:**

Should you be overriding LLMInferenceService scheduler arguments, note that in version 3.0 and later:

1. Arguments use different naming pattern:
   - **Old:** camel case (e.g., `--certPath`)
   - **New:** dash case/kebab-case (e.g., `--cert-path`)

2. Default TLS certificate path has relocated:
   - **Old:** `/etc/ssl/certs`
   - **New:** `/var/run/kserve/tls`

3. Signed TLS certificates (by OpenShift service signer) are **mandatory**
   - Therefore `--cert-path` argument is mandatory

4. Volume mount configuration must match the new location:

```yaml
kind: LLMInferenceService
spec:
  router:
    scheduler:
      containers:
      - name: main
        args:
          # ... other arguments omitted ...
          - "--cert-path"
          - "/var/run/kserve/tls"
        volumeMounts:
          - mountPath: /var/run/kserve/tls
            name: tls-certs
            readOnly: true
```

---

#### Model Serving Runtimes

**Scan for Deprecated Runtimes:**

Use this script to scan an OpenShift namespace for deployed KServe InferenceServices and display their associated ServingRuntimes and Runtime images:

```bash
# Download the script
curl -fsSL https://gist.githubusercontent.com/Raghul-M/a1412ea74573fb05e16edf1afd557413/raw/serving-runtime.sh -o serving-runtime.sh

# Make it executable
chmod +x serving-runtime.sh

# Run the scan
./serving-runtime.sh -n <namespace> --verbose
```

**Verification:**

Make sure the Output Deployments are **not using** the Removed Serving Runtimes:
- `ovms` (OpenVINO Model Server)
- `caikit-standalone-serving-template`
- `caikit-tgis-serving-template`

---

#### Workbenches/Notebooks Server

**Admin Actions:**

1. **Notify users** to update their workbenches to the latest versions (2025.2)

2. **Ensure users save their work:**
   - All users must save their work to their Persistent Volume Claims (PVCs)

3. **Stop all notebooks:**
   - Ensure users stop their notebooks before proceeding with the operator upgrade
   - **Alternative:** If users prefer to have their notebooks in running state, notify them that after upgrade of OpenShift AI operator, they would have to perform the after-upgrade process to get their notebooks upgraded to new Auth-compatible content

---

#### Ray Training Operator

**Before upgrading RHOAI,** run the pre-upgrade command to:
- Verify your cluster is ready for the upgrade
- Back up your RayCluster CR configurations (yaml only)

```bash
python ray_cluster_migration.py pre-upgrade
```

You'll be prompted for a backup directory:

```
Enter backup directory [./raycluster-backups]:
```

**What Happens:**

1. **Pre-flight checks run automatically**

**Failure Example:**

```
Running pre-upgrade checks...
------------------------------------------------------------
  [FAIL] Permissions: Missing permissions
       - List namespaces: OK
       - List RayClusters: OK
       - Get RayClusters: OK
       - Update RayClusters: DENIED
       Missing permissions: Update RayClusters: DENIED
  [FAIL] cert-manager: cert-manager not detected
       cert-manager is required for RHOAI 3.x. Install it via OperatorHub before proceeding with the upgrade.
  [FAIL] codeflare-operator: codeflare is Managed in DSC (should be Removed)
       Set codeflare to Removed in your DataScienceCluster before upgrading:
       oc patch datasciencecluster <name> --type merge -p '{"spec":{"components":{"codeflare":{"managementState":"Removed"}}}}'
------------------------------------------------------------

Pre-upgrade checks failed. Please resolve the issues above before
proceeding with the RHOAI upgrade.

WARNING: Proceeding with the RHOAI upgrade without resolving these issues
may result in your Ray infrastructure becoming unavailable or unrecoverable.
```

**Success Example:**

```
Running pre-upgrade checks...
------------------------------------------------------------
  [OK] Permissions: All required permissions granted
  [OK] cert-manager: cert-manager CRD found
  [OK] codeflare-operator: codeflare is Removed in DSC 'default-dsc'
------------------------------------------------------------
All pre-upgrade checks passed.
```

2. **If pre-requisite checks pass, backups are created** for all your RayCluster CRs (yaml only):

```
Backing up 3 RayCluster(s) (all clusters across all namespaces)

  Backed up: production-cluster (namespace: production) -> ./raycluster-backups/raycluster-production-cluster-production.yaml
  Backed up: staging-cluster (namespace: staging) -> ./raycluster-backups/raycluster-staging-cluster-staging.yaml
  Backed up: dev-cluster (namespace: dev) -> ./raycluster-backups/raycluster-dev-cluster-dev.yaml

Backup complete: 3 RayCluster(s) saved to ./raycluster-backups

Next steps:
  1. Perform the RHOAI upgrade
  2. Run 'post-upgrade' to migrate the RayClusters
```

**⚠️ Important:** These files are your backups. It is your responsibility to ensure they are stored in a safe place.

---

#### TrustyAI

**GuardrailsOrchestrator Preparation:**

1. **Validate your model deployment:**
   - Ensure you have a LLM deployed on vLLM ServingRuntime in your namespace
   - Optionally, if you're interested in using your own detector, ensure you have that deployed in the same namespace as your model

2. **Validate your GuardrailsOrchestrator ConfigMap:**
   - Ensure `chat_generation.service.hostname` and `chat_generation.service.port` fields are defined
   - Ensure each of your detectors have their `.type`, `.service.hostname`, `service.port`, `chunker_id`, and `default_threshold` fields defined

3. **Validate your GuardrailsOrchestrator Custom Resource:**
   - Ensure `spec.orchestratorConfig` is defined and its value matches the name of the GuardrailsOrchestrator ConfigMap
   - Ensure `spec.replicas` is at least 1
   - **Recommended:** Set `spec.enableGuardrailsGateway` and `spec.enableBuiltInDetectors` to `true`
   - If `spec.enableGuardrailsGateway` is `true`, ensure you have an additional ConfigMap for gateway configuration and the `spec.guardrailsGatewayConfig` value set to the ConfigMap's name

4. **Remove the spec.otelExporter section in CR if present**

5. **Optionally configure OpenTelemetry:**
   - If interested in collecting OpenTelemetry metrics and traces from GuardrailsOrchestrator instances:
     - Install: Tempo Operator and Red Hat build of OpenTelemetry
     - Deploy: TempoStack and OpenTelemetry CRs in the same namespace as your model
   - **Note:** Since TrustyAI does not manage the Tempo and OpenTelemetry Operators, consult official Red Hat documentation
   - **Important:** Remove the `spec.otelExporter` section from your GuardrailsOrchestrator CRs (breaking changes in RHOAI 3.x)

---

#### LlamaStack

**Required DSC Configuration for Successful Upgrade:**

- RHOAI 2.25 is installed, CSV status is "Succeeded", DSCI and DSC are "Ready"
- InstallPlan of RHOAI 2.25 is set to Manual (not required but prevents accidental automatic upgrade)
- If any of the following operators are currently configured as Managed, consult specific upgrade documentation:
  - CodeFlare
  - Kueue
  - Ray
  - MLFlow Operator

**Audit Requirements:**

1. **Audit VectorDB API usage:**
   - Identify all applications and RAG pipelines that use the deprecated VectorDB API
   - Record these usages to plan mandatory post-upgrade code changes

2. **Audit llama-stack-client usage:**
   - Identify any usage of `llama-stack-client` version 0.2.x
   - Identify usage of deprecated or removed APIs
   - All such usages require code changes when upgrading to client version 0.4.x

3. **Verify RBAC for Llama Stack access:**
   - Ensure RHOAI users have RBAC permissions to access required InferenceService instances
   - Verify access paths used by Llama Stack and the GenAI Playground
   - This check mitigates access failures caused by stricter authentication enforcement in 3.3

**Audit Config Changes Required:**

1. **VLLM Inference Provider:**
   - In 2.25: `VLLM_URL` defaulted to localhost, vllm-inference provider always enabled
   - In 3.3: Provider disabled if `VLLM_URL` environment variable isn't set
   - **Action:** Users must explicitly set `VLLM_URL` (in LlamaStackDistribution CR) to enable the vllm-inference provider

2. **Embedding Model Configuration:**
   - In 3.3, embedding models require separate configuration to replace inline sentence-transformers provider

   **Required environment variables:**
   - `VLLM_EMBEDDING_URL` - URL for the embedding model endpoint
   - `VLLM_EMBEDDING_API_TOKEN` - API token for the embedding endpoint

   **Optional environment variables (with defaults):**
   - `VLLM_EMBEDDING_MAX_TOKENS` - Maximum tokens for embeddings (default: 4096)
   - `VLLM_EMBEDDING_TLS_VERIFY` - TLS verification setting (default: true)
   - `EMBEDDING_MODEL` (default: granite-embedding-125m-english)
   - `EMBEDDING_PROVIDER` (default: vllm-embedding)
   - `EMBEDDING_PROVIDER_MODEL_ID` (default: ibm-granite/granite-embedding-125m-english)
   - `EMBEDDING_DIMENSION` (default: 768)

3. **PostgreSQL Storage Backend (Required):**
   - In 3.3, SQLite-based storage has been replaced with PostgreSQL backend
   - Users must configure PostgreSQL connection parameters

   **Environment variables:**
   - `POSTGRES_HOST` (default: localhost)
   - `POSTGRES_PORT` (default: 5432)
   - `POSTGRES_DB` (default: llamastack)
   - `POSTGRES_USER` (default: llamastack)
   - `POSTGRES_PASSWORD` (default: llamastack)
   - `POSTGRES_TABLE_NAME` (default: llamastack_kvstore)

4. **Sentence Transformers Provider:**
   - In 2.25: sentence-transformers provider was always enabled
   - In 3.3: Must be explicitly enabled by setting `ENABLE_SENTENCE_TRANSFORMERS` environment variable

5. **AWS Bedrock Authentication:**
   - AWS Bedrock authentication has been simplified
   - Instead of multiple AWS credential environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_PROFILE`, etc.)
   - Use:
     - `AWS_BEARER_TOKEN_BEDROCK` - Bearer token for Bedrock authentication
     - `AWS_DEFAULT_REGION` - AWS region (default changed to: us-east-2)

6. **Telemetry Environment Variables:**
   - The following telemetry-related environment variables are no longer supported in 3.3:
     - `OTEL_SERVICE_NAME`
     - `TELEMETRY_SINKS`
     - `OTEL_EXPORTER_OTLP_ENDPOINT`

**Config File Name/Format Changes:**

Users who used a custom `run.yaml` in their RHOAI 2.25.x deployment should convert from the old to the new format. This includes:
- Structural Changes: Storage backend migration to PostgreSQL from sqlite
- Telemetry removal
- Provider configuration updates
- Reorganization of registered_resources in run.yaml
- **Rename `run.yaml` to `config.yaml`**

**Seamless Upgrade Approach:**

To enable a seamless upgrade, you can include both the old `run.yaml` and new `config.yaml` in the same ConfigMap. The operator will automatically switch from one to the other during the upgrade.

This approach allows:
- RHOAI 2.25 to continue using `run.yaml`
- RHOAI 3.3 to automatically switch to `config.yaml` after upgrade
- No service interruption during the transition

**Note:** Users who used the default configuration do not need to go through this step.

**Setup LlamaStackDistribution CR:**

**Step 1: Create Project and Secrets**

Create new project (namespace):

```bash
export MY_PROJECT="llama-stack-2-25-to-3-upgrade"
oc new-project "$MY_PROJECT"
```

**NOTE:** This name will be used throughout this guide to simplify variable referencing.

Configure the VLLM provider to connect to a VLLM-compatible inference endpoint.

Update the following variables to match your inference provider settings (the same provider you've been using with RHOAI 2.25):

```bash
export MY_PROJECT="llama-stack-2-25-to-3-upgrade"
export INFERENCE_MODEL="<YOUR_INFERENCE_MODEL>"
export VLLM_URL="<YOUR_VLLM_URL>"
export VLLM_TLS_VERIFY="false"
export VLLM_API_TOKEN="<YOUR_VLLM_API_TOKEN>"
```

Create secret:

```bash
oc create secret generic llama-stack-inference-model-secret \
  --from-literal INFERENCE_MODEL="$INFERENCE_MODEL" \
  --from-literal VLLM_URL="$VLLM_URL" \
  --from-literal VLLM_TLS_VERIFY="$VLLM_TLS_VERIFY" \
  --from-literal VLLM_API_TOKEN="$VLLM_API_TOKEN"
```

**Step 2: Create LlamaStackDistribution with 2.25 relevant config**

Save as `llama-stack-upgrade-test.yaml`:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llama-stack-upgrade-test
spec:
  replicas: 1
  server:
    containerSpec:
      env:
        - name: INFERENCE_MODEL
          valueFrom:
            secretKeyRef:
              key: INFERENCE_MODEL
              name: llama-stack-inference-model-secret
        - name: VLLM_URL
          valueFrom:
            secretKeyRef:
              key: VLLM_URL
              name: llama-stack-inference-model-secret
        - name: VLLM_TLS_VERIFY
          valueFrom:
            secretKeyRef:
              key: VLLM_TLS_VERIFY
              name: llama-stack-inference-model-secret
        - name: VLLM_API_TOKEN
          valueFrom:
            secretKeyRef:
              key: VLLM_API_TOKEN
              name: llama-stack-inference-model-secret
        - name: MILVUS_DB_PATH
          value: ~/.llama/milvus.db
        - name: FMS_ORCHESTRATOR_URL
          value: 'http://localhost'
      name: llama-stack
      port: 8321
    distribution:
      name: rh-dev
    storage:
      size: 5Gi
```

Apply:

```bash
oc apply -f llama-stack-upgrade-test.yaml
```

Make sure Llama-Stack pod is up and running successfully:

```bash
oc get pods
# Expected output:
# NAME                                       READY   STATUS    RESTARTS   AGE
# llama-test-2-25-upgrade-5c9d6549dc-pdrrn   1/1     Running   0          14m
```

**Before Upgrade Steps:**

**Requirement:** The PostgreSQL Operator version 14 or later has been installed within your cluster.

**Step 3: Update LlamaStackDistribution CR**

Before upgrading the operator, update your LlamaStackDistribution CR to include the required configuration changes.

Edit your LlamaStackDistribution:

```bash
oc edit llamastackdistribution llama-test-2-25-upgrade
```

Add the required environment variables to `spec.server.containerSpec.env`:

**Example PostgreSQL Configuration (Required):**

```yaml
- name: POSTGRES_HOST
  value: postgres.llama-stack-2-25-to-3-upgrade.svc.cluster.local  # or your PostgreSQL service name
- name: POSTGRES_PORT
  value: "5432"  # or your PostgreSQL service port
- name: POSTGRES_DB
  value: "postgres"  # or your PostgreSQL db name
- name: POSTGRES_USER
  value: "postgres"  # or your PostgreSQL user
- name: POSTGRES_PASSWORD
  value: "postgres"  # or your PostgreSQL password
```

**Example Embedding Model Configuration (Required):**

```yaml
- name: ENABLE_SENTENCE_TRANSFORMERS
  value: "true"
- name: EMBEDDING_PROVIDER
  value: "sentence-transformers"
```

Save and exit the editor.

---

#### vLLM/RHAIIS

Before upgrading from Red Hat OpenShift AI 2.25 to 3.3, review the following changes to the vLLM model-serving runtime (Red Hat AI Inference Server).

**Customers who were using the default vLLM V1 engine in RHOAI 2.25 are not affected and do not need to take any action.**

**Check if you explicitly enabled the vLLM V0 engine:**

In RHOAI 2.25 (RHAIIS 3.2.2), the vLLM V1 engine was the default. However, customers could switch to the V0 engine by setting the environment variable `VLLM_USE_V1=0` on their model server.

Starting with RHOAI 3.0 (RHAIIS 3.2.3), the V0 engine has been completely removed and the `VLLM_USE_V1=0` option is no longer supported.

**If you did not set this variable, you were already using V1 and are not affected.**

To check whether any of your deployed model servers use this setting:

**Using the OpenShift AI dashboard:**
- In your data science project, navigate to the Models tab
- For each model deployed with the vLLM serving runtime, review the environment variables configured for the model server
- Check whether `VLLM_USE_V1` is set to 0

**Using the OpenShift CLI:**

```bash
oc get inferenceservice -A -o yaml | grep -A1 "VLLM_USE_V1"
```

**Check if you are serving any unsupported encoder-decoder models:**

With the removal of the V0 engine, the following encoder-decoder model architectures are no longer supported:
- BartForConditionalGeneration
- MBartForConditionalGeneration
- DonutForConditionalGeneration
- Florence2ForConditionalGeneration
- MllamaForConditionalGeneration

If you are currently serving any of these models on the vLLM serving runtime, they will not work after the upgrade.

**Note:** BART model support is planned to be reinstated in a future Red Hat AI Inference Server release.

**Note:** No changes to the DataScienceCluster (DSC) resource are required for this component.

---

## During Upgrade

### AI Core Platform

1. **Monitor RHOAI operator pods**
   - They should begin restarting to replace the 2.25 operator
   - Operator Pods should eventually be running and have the condition "Ready" set to "True"

### Workbenches/Notebooks Server

1. **Monitor and wait** for the RHOAI operator to complete its upgrade
2. **Wait** for the notebook-controller component to be updated

**⚠️ Important:** Do not start any notebooks until the upgrade is complete and post-upgrade steps are performed.

### AI Hub

During upgrade:
- Model catalog UI may become inaccessible briefly while catalog pods are respun
- Same for model registry UI page

### Dashboard

**Note:** During the upgrade, the RHOAI Dashboard will become inaccessible. The user will see a "General loading error" with message: "Unknown error occurred during startup. Logging out and logging back in may solve the issue."

Follow the steps below once the upgrade has completed in order to navigate to the new Dashboard route.

---

## After Upgrade

### Prerequisites

#### Kueue

If DSC condition `status.conditions.Type=KueueReady` has `Status=False` with message `"Kueue managementState Managed is not supported, please use Removed or Unmanaged"`, it means Kueue wasn't migrated to RHBoK as required by Before Upgrade prerequisites.

**Recovery:** Please follow the steps and caveats in the Before Upgrade section to recover.

#### AI Core Platform

- RHOAI 3.3 upgrade is finished
- Only RHOAI 3.3 is installed (apart from dependent operators) in the "Installed Operators" window, its CSV status is "Succeeded"
- RHOAI 2.25 is not present in the "Installed Operators" window

#### TrustyAI

- OpenShift Cluster with Red Hat OpenShift AI (RHOAI) installed via Data Science Cluster (DSC) custom resource
- DSC must have TrustyAI set to `Managed`
- `oc` and cluster admin access
- A LLM deployed on vLLM ServingRuntime in your working namespace
- A ConfigMap with GuardrailsOrchestrator configuration
- A GuardrailsOrchestrator instance

---

### Procedure

#### AI Core Platform

**Optionally install user CRs** if their functionality is used:

1. **If MaaS is to be set to Managed:**
   - MaaS gateway

2. **If Trainer v2 is to be set to Managed:**
   - JobSetOperator CR

---

#### Workbenches/Notebooks Server

**Update Custom Workbench Images (if applicable):**

Custom images require the following updates to be compatible with 3.0:

1. **Configure path-based routing:**
   - Your workbench must serve all content from `${NB_PREFIX}` (e.g., `/notebook/<namespace>/<workbench-name>`)
   - Requests to paths outside this prefix (like `/index.html` or `/api/data`) will not be routed to your workbench container
   - Update your application to handle requests at `${NB_PREFIX}/...` paths

2. **Configure base path:**
   - FastAPI: `FastAPI(root_path=os.getenv('NB_PREFIX', ''))`
   - OR update nginx to preserve the prefix in redirects

3. **Implement health endpoints:**
   - `${NB_PREFIX}/api` → HTTP 200
   - `${NB_PREFIX}/api/kernels` → HTTP 200
   - `${NB_PREFIX}/api/terminals` → HTTP 200

4. **Use relative URLs:**
   - No hardcoded absolute paths like `/menu`

**Reference:** For more information, read the migration doc: [Gateway API Migration Guide](https://github.com/opendatahub-io/notebooks/blob/main/docs/gateway-api-migration-guide.md)

**Update OpenShift AI based Code-Server and RStudio Images:**

- **For Jupyterlab based images:** No additional action required, apart from the later described steps

- **CodeServer:**
  - Users on `2025.1` image tag must upgrade to `2025.2` image tag by updating their workbench image
  - The `2025.1` tag is deprecated, no longer supported, and will not be accessible on the Dashboard UI
  - If users choose to continue on an older image tag and upgrade the workbench to new auth standard, the image would not be able to establish connection with the auth layer and would be redirected to gateway URL

- **RStudio:**
  - Run a new Build to update RStudio Image tag
  - If users choose to continue on an older image tag and upgrade the workbench to new auth standard, the image would not be able to establish connection with the auth layer and would be redirected to gateway URL

**Migrate Existing Notebooks:**

**RHOAI Admin / Cluster Admin:**

Admin can execute the following script to upgrade workbenches for new auth compatibility:

```bash
# Variables
NAME="<VALUE>"
NAMESPACE="<VALUE>"

# Generate the patch string dynamically
PATCH=$(oc get notebook $NAME -n $NAMESPACE -o json | jq -c '
[
  {"op":"add","path":"/metadata/annotations/notebooks.opendatahub.io~1inject-auth","value":"true"},
  {"op":"remove","path":"/metadata/annotations/notebooks.opendatahub.io~1inject-oauth"},
  {"op":"remove","path":"/metadata/annotations/notebooks.opendatahub.io~1oauth-logout-url"},
  (
    .spec.template.spec.containers | to_entries[] |
    select(.value.name == "oauth-proxy") |
    {"op":"remove", "path": "/spec/template/spec/containers/\(.key)"}
  ),
  (
    .metadata.finalizers | to_entries[] |
    select(.value == "notebook-oauth-client-finalizer.opendatahub.io") |
    {"op":"remove", "path": "/metadata/finalizers/\(.key)"}
  ),
  (
    .spec.template.spec.volumes | to_entries[] |
    select(.value.name | IN("oauth-config", "oauth-client", "tls-certificates")) |
    {"op":"remove", "path": "/spec/template/spec/volumes/\(.key)"}
  )
] | sort_by(.path) | reverse')

# Execute the patch
oc patch notebook $NAME -n $NAMESPACE --type='json' -p="$PATCH"
```

**Users:**

User can perform upgrade of their workbenches in these scenarios:
- User opted for having their workbenches in running state during upgrade
- User workbench was missed by admin to be upgraded

**User with Access to Dashboard UI:**

**Option 1:**
- Edit the workbench description in the Dashboard and save
- This triggers a restart and redeploy with the correct sidecar
- Dashboard recreates the Notebook CR automatically

**Option 2: Delete and Recreate**

Delete the Notebook:

```bash
oc delete notebook <NAME> -n <NAMESPACE>
```

Recreate the notebook resource using the Dashboard.

---

## Verification Procedures

### AI Core Platform

Verify the following:

1. **Check RHOAI 2.25 CSV status is "Succeeded"**
2. **Check DSCI and DSC are "Ready"**, their status sections don't show any errors
3. **Check all operator pods** in the operator namespace (`redhat-ods-operator` by default) are running and their condition "Ready" is "True"
4. **Check all component controller pods** in the applications namespace (`redhat-ods-applications` by default) are running and their condition "Ready" is "True"

---

### Model Serving

1. **Verify RawDeployment Conversion:**
   - Ensure no InferenceServices are running in Serverless or ModelMesh mode
   - Run the following command and confirm that the MODE column shows `RawDeployment` for all services:

```bash
oc get inferenceservices --all-namespaces -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,MODE:.metadata.annotations.'serving\.kserve\.io/deploymentMode'
```

2. **Verify DSC Configuration:**
   - Confirm that the Data Science Cluster (DSC) is configured to manage KServe in RawDeployment mode
   - Confirm that the legacy Serving component is removed

```bash
oc get dsc default-dsc -o jsonpath='{.spec.components.kserve}'
```

3. **Verify Service Mesh Removal (DSCI):**
   - Confirm that Service Mesh is no longer managed by the DSC Initialization (DSCI)

```bash
oc get dsci default-dsci -o jsonpath='{.spec.serviceMesh}'
```

4. **Verify Operator Removal:**
   - Ensure that the Serverless and Service Mesh v2 operators are no longer present (unless required by non-RHOAI workloads)

```bash
oc get subscriptions.operators.coreos.com --all-namespaces
```

---

### Workbenches/Notebooks Server

1. **Verify all workbench images are updated to version 2025.2**
2. **Confirm all user data is saved to PVCs**
3. **Verify all notebooks are in a stopped state across all namespaces:**

```bash
oc get notebooks --all-namespaces
```

---

### TrustyAI

**GuardrailsOrchestrator:**

1. **Validate your LLM deployment** in your namespace by sending some inference requests to it:

Port forward your model pod:

```bash
oc port-forward $(oc get pods -o name | grep <model_name>) 8080:8080
```

2. **Open a separate terminal window and send an inference request to the model:**

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d "{\"model\": \"<model_name>\", \"messages\": [{\"content\": \"Hi, can you tell me about yourself?\", \"role\": \"user\"}]}"
```

3. **Remove the spec.otelExporter section in CR if present:**

Double check that the `spec.otelExporter` section has been removed from your GuardrailsOrchestrator CRs:

```bash
oc get guardrailsorchestrator <guardrailsorchestrator_name> -o jsonpath='{.spec.otelExporter}'
```

---

### AI Hub

If any model registry or custom catalog source was created pre-upgrade, ensure they are functional before upgrading RHOAI Operator.

---

### Feature Store

**If you have Feature Store CRD instance created (Optional):**

1. As an OpenShift AI admin, verify created Feast Feature Store Instance is in Ready state:

```bash
oc get featurestores
```

2. As an OpenShift AI admin, verify Cronjob for the feast instance created to perform feast apply and materialize is successful

3. As a Data Scientist, verify RHOAI Dashboard Feature Store UI shows all the features, entities, feature-views, data sources, feature services listing correctly

---

### LlamaStack

**Test API Endpoint:**

Test chat completions API for enabled Inference provider (vLLM):

1. **Rsh to pod** (use your pod reference here):

```bash
oc rsh pods/llama-test-2-25-upgrade-5c9d6549dc-pdrrn
```

2. **Test the endpoint:**

Ensure the response returns an HTTP/1.1 200 OK status and contains the expected content.

**NOTE:** The model used here, `llama-3-2-3b`, is from the vllm-inference provider. Ensure you replace this with the details relevant to your specific inference provider.

```bash
curl -v --request POST \
  --url http://0.0.0.0:8321/v1/inference/completion \
  --header 'content-type: application/json' \
  --data '{
  "model_id": "vllm-inference/llama-3-2-3b",
  "content": "The future of artificial intelligence is",
  "max_tokens": 50
}'
```

**Expected outcome** (although the Language Model may yield unanticipated results):

```
* Connected to 0.0.0.0 (127.0.0.1) port 8321 (#0)
> POST /v1/inference/completion HTTP/1.1
> Host: 0.0.0.0:8321
> User-Agent: curl/7.76.1
> Accept: */*
> content-type: application/json
> Content-Length: 124
>
* Mark bundle as not supporting multiuse
< HTTP/1.1 200 OK
< date: Thu, 05 Feb 2026 19:44:47 GMT
< server: uvicorn
< content-length: 2268
< content-type: application/json
< x-trace-id: 1ff4847d44d2ecee984c626da0499fa0
<
{"metrics":[{"metric":"prompt_tokens","value":7,"unit":null},{"metric":"completion_tokens","value":376,"unit":null},{"metric":"total_tokens","value":383,"unit":null}],"content":" bright, and it's being shaped by the work of researchers and developers around the world..."}
```

---

### vLLM/RHAIIS

1. **Confirm** that none of your deployed model servers have the `VLLM_USE_V1=0` environment variable set
   - If the variable is not present or is set to 1, no action is required

2. **Confirm** that none of your deployed models use the unsupported encoder-decoder architectures listed above
   - You can check the model architecture by inspecting the `config.json` file of each model in its storage location for the `architectures` field

---

### Workbenches/Notebooks Server

1. **Confirm the RHOAI operator upgrade has completed successfully**
2. **Verify the notebook-controller component has been updated**

---

## Related Documentation

- **Troubleshooting Guide:** [UPGRADE-TROUBLESHOOTING-2.25.2-to-3.3.0.md](UPGRADE-TROUBLESHOOTING-2.25.2-to-3.3.0.md)
  - Operational fixes for upgrade errors
  - Component-specific failure scenarios
  - Recovery procedures

- **Official RHOAI Documentation:**
  - [Red Hat OpenShift AI Self-Managed Documentation](https://docs.redhat.com/en/documentation/red_hat_openshift_ai_self-managed/)

---

**Document Version:** 1.0
**Last Updated:** 2026-02-10
**Source:** Engineering Migration Guide (copied 2026-02-09 17:00 EST)
**Upgrade Path:** RHOAI 2.25.2 → 3.3.0
