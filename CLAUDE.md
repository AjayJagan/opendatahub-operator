# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The **opendatahub-operator** is a Kubernetes operator that manages Open Data Hub (ODH) and Red Hat OpenShift AI (RHOAI) platform deployments. It orchestrates data science components like Jupyter Notebooks, Data Science Pipelines, KServe, and more through declarative CRDs.

**Key characteristics:**
- Single codebase supporting both ODH (upstream) and RHOAI (downstream) variants using Go build tags
- Action-based reconciliation architecture with reusable components
- Component registry pattern for extensibility
- Kustomize-based manifest deployment with platform-specific overlays
- Generic reconciler framework using Go generics for type safety
- Server-Side Apply (SSA) as default deployment strategy
- 18+ managed components (dashboards, model serving, pipelines, training, etc.)

## Common Development Commands

### Build and Test

```bash
# Build the operator binary
make build

# Run unit tests
make unit-test

# Run e2e tests (requires cluster access via KUBECONFIG)
make e2e-test

# Run e2e tests for specific component only
make e2e-test -e E2E_TEST_COMPONENT=dashboard

# Run e2e tests excluding specific components
make e2e-test -e E2E_TEST_COMPONENT=!ray

# Run Prometheus alert unit tests
make test-alerts

# Check for alerts missing unit tests
make check-prometheus-alert-unit-tests

# Lint code
make lint

# Auto-fix linting issues
make lint-fix

# Format code
make fmt
```

### Local Development

```bash
# Fetch component manifests from remote repositories
make get-manifests

# Run operator locally (with webhooks)
make run

# Run operator locally without webhooks (useful for debugging)
make run-nowebhook

# Generate CRDs, code, and API docs after API changes
make generate manifests api-docs

# Update API documentation only
make api-docs
```

### Build and Deploy

```bash
# Build operator image (defaults to quay.io/opendatahub/opendatahub-operator:latest)
make image-build IMG=quay.io/<username>/opendatahub-operator:<tag>

# Build with local manifests instead of fetching from remote
make image-build USE_LOCAL=true

# Build RHOAI variant
make image-build ODH_PLATFORM_TYPE=rhoai

# Deploy operator to cluster
make deploy IMG=<your-image> OPERATOR_NAMESPACE=<namespace>

# Undeploy operator
make undeploy

# Install CRDs only
make install

# Uninstall CRDs
make uninstall
```

### Bundle and Catalog (OLM)

```bash
# Generate operator bundle
make bundle

# Build and push bundle image
make bundle-build bundle-push BUNDLE_IMG=<registry>/<bundle-image>:<version>

# Build catalog image (for operator upgrade testing)
make catalog-build catalog-push \
  -e CATALOG_IMG=<registry>/<catalog>:<version> \
  BUNDLE_IMGS=<bundle1>,<bundle2>,<bundle3>
```

## Architecture

### Core CRDs and Their Relationship

**DSCInitialization** (`api/dscinitialization/v2/`)
- **Purpose:** Platform-wide initialization and infrastructure setup
- **Singleton:** Only one instance per cluster
- **Creates:**
  - Application namespace (default: `opendatahub` for ODH, `redhat-ods-applications` for RHOAI)
  - ConfigMaps (monitoring configurations, platform configs)
  - NetworkPolicies (security policies)
  - Service CRs (Monitoring, Auth, GatewayConfig)
  - Default HardwareProfile
- **Cleanup Tasks:**
  - Removes old ServiceMesh FeatureTrackers (from 2.x → 3.x upgrades)
- **Must be created before DataScienceCluster**
- **Version:** v2 is storage version, v1 served for backwards compatibility

**DataScienceCluster** (`api/datasciencecluster/v2/`)
- **Purpose:** Declarative component deployment configuration
- **Singleton:** Only one instance per cluster
- **Creates:** Component CRs based on `.spec.components`
- **Each component has:**
  - `managementState`: `Managed`, `Removed`, or `Unmanaged`
  - Component-specific configuration fields
- **Status aggregates:** Status from all component CRs
- **Version:** v2 is storage version, v1 served for backwards compatibility

**Component CRs** (`api/components/v1alpha1/`)
- Individual CRs for each component (18 total):
  - **UI & Development**: Dashboard, Workbenches
  - **Model Serving**: Kserve, ModelController, ModelRegistry
  - **Training**: Trainer, TrainingOperator
  - **Pipelines**: DataSciencePipelines
  - **Distributed Computing**: Ray, SparkOperator (newly added)
  - **ML Tools**: MLflowOperator (newly added), FeastOperator
  - **Job Management**: Kueue
  - **AI Features**: TrustyAI, LlamaStackOperator
  - **Legacy**: CodeFlare (inactive), ModelMeshServing (deprecated)
- Owned by DataScienceCluster
- Reconciled by their respective component controllers

**Service CRs** (`api/services/v1alpha1/`)
- `Monitoring`: Metrics (Prometheus), traces (Tempo), alerting, Perses dashboards
- `Auth`: Authentication configuration, kube-auth-proxy with HPA support
- `GatewayConfig`: Ingress gateway routing with:
  - IngressMode (OcpRoute or LoadBalancer)
  - OIDC configuration
  - Custom TLS certificate support
  - Network policy configuration
  - AuthProxyTimeout settings
- ~~`ServiceMesh`~~: **REMOVED in 3.0** (type exists for backwards compatibility only)

### Reconciliation Flow

```
User creates DSCInitialization
  └─> DSCI Controller reconciles:
      ├─> Creates application namespace
      ├─> Creates ConfigMaps and NetworkPolicies
      ├─> Cleans up old ServiceMesh resources (from 2.x upgrades)
      ├─> Creates Monitoring CR
      ├─> Creates Auth CR
      ├─> Creates GatewayConfig CR
      └─> Creates default HardwareProfile

User creates DataScienceCluster
  └─> DSC Controller reconciles:
      ├─> Verifies DSCI exists
      ├─> Creates component CRs (Dashboard, Kserve, etc.)
      └─> Updates DSC status from component statuses

Component Controller (e.g., Dashboard) reconciles:
  ├─> initialize() - Sets manifest paths
  ├─> kustomize.NewAction() - Renders Kustomize manifests
  ├─> deploy.NewAction() - Applies resources via Server-Side Apply
  ├─> deployments.NewAction() - Checks deployment readiness
  ├─> updateStatus() - Updates component CR status
  └─> gc.NewAction() - Garbage collects orphaned resources
```

### Controller Architecture

**Generic Reconciler Framework** (`pkg/controller/reconciler/`)
- Type-safe, generic reconciler using Go generics
- Action-based pipeline pattern
- Each controller defines a sequence of actions

**Action Pattern** (`pkg/controller/actions/`)
Actions are composable functions with signature:
```go
func(ctx context.Context, rr *types.ReconciliationRequest) error
```

**Common Actions:**
- `render/kustomize`: Render Kustomize manifests with overlays
- `deploy`: Apply resources using Server-Side Apply (SSA) or Patch mode
- `gc`: Garbage collect orphaned resources (must be last action)
- `status/deployments`: Update status based on deployment readiness
- `dependency`: Check for external operator dependencies
- `deleteresource`: Delete specific resources
- `sanitycheck`: Validate configurations
- `cacher/resourcecacher`: Cache resources for performance

**Component-specific actions** are defined in `internal/controller/components/<component>/<component>_controller_actions.go`

### Component Registry Pattern

**Location:** `internal/controller/components/registry/`

**How it works:**
1. Each component implements `ComponentHandler` interface:
   ```go
   type ComponentHandler interface {
       GetName() string
       Init(platform) error
       NewCRObject(dsc) common.PlatformObject
       NewComponentReconciler(ctx, mgr) error
       UpdateDSCStatus(ctx, rr) error
   }
   ```

2. Components self-register using `init()`:
   ```go
   func init() {
       cr.Add(&componentHandler{})
   }
   ```

3. DSC controller discovers components from registry

**Adding a new component:**
- Use `make new-component COMPONENT=<name>` to scaffold
- Implement `ComponentHandler` interface
- Add reconciler with action pipeline
- Import in `cmd/main.go`

See [docs/COMPONENT_INTEGRATION.md](docs/COMPONENT_INTEGRATION.md) for detailed integration steps.

### Platform Variants (ODH vs RHOAI)

The operator supports **three platforms** using Go build tags:

**Platforms:**
- `OpenDataHub` - Community upstream (build tag: `odh` or none)
- `SelfManagedRhoai` - Red Hat OpenShift AI on-prem (build tag: `rhoai`)
- `ManagedRhoai` - Red Hat OpenShift AI cloud/addon (build tag: `rhoai`)

**Build tag files:**
- `//go:build !rhoai` - ODH-specific code
- `//go:build rhoai` - RHOAI-specific code
- No tag - shared code

**Key differences:**
- Default namespaces (ODH: `opendatahub`, RHOAI: `redhat-ods-applications`)
- Manifest overlays (`opt/manifests/<component>/odh/` vs `rhoai/onprem/` vs `rhoai/addon/`)
- Feature flags (e.g., SegmentIO telemetry in self-managed RHOAI)

**Configuration in Makefile:**
```makefile
ODH_PLATFORM_TYPE=OpenDataHub  # or rhoai
```

### Manifest Management

**Location:** `opt/manifests/` (embedded in operator image)

**Structure:**
```
opt/manifests/<component>/
├── base/                    # Base Kubernetes resources
├── overlays/
│   ├── odh/                 # ODH-specific overlay
│   ├── rhoai/
│   │   ├── addon/           # Managed RHOAI overlay
│   │   └── onprem/          # Self-managed RHOAI overlay
└── params.env               # Image references and env vars
```

**How manifests are deployed:**
1. Component's `Init()` method calls `ApplyParams()` to replace image environment variables in `params.env`
2. Kustomize action renders overlays based on platform type
3. Deploy action applies rendered manifests using Server-Side Apply (SSA)
4. GC action removes resources not in current manifest set

**Fetching manifests:**
- `make get-manifests` - Fetches from remote repos (defined in `get_all_manifests.sh`)
- `make image-build USE_LOCAL=true` - Uses local `opt/manifests/` instead of fetching

**Customizing manifest source:**
```bash
./get_all_manifests.sh --dashboard="org:repo:branch:src:dest"
```

## Key Code Locations

### API Definitions
- `api/common/` - Shared types (Status, Condition, Platform)
- `api/components/v1alpha1/` - Component CRDs (18 component types)
- `api/datasciencecluster/v2/` - DataScienceCluster CRD (storage version)
- `api/datasciencecluster/v1/` - DataScienceCluster CRD (served for backwards compatibility)
- `api/dscinitialization/v2/` - DSCInitialization CRD (storage version)
- `api/dscinitialization/v1/` - DSCInitialization CRD (served for backwards compatibility)
- `api/services/v1alpha1/` - Service CRDs (Monitoring, Auth, GatewayConfig)
- `api/features/v1/` - FeatureTracker CRD (LEGACY - for cleanup only, removed in 3.0)
- `api/infrastructure/v1/` - HardwareProfile, Certificate (ServiceMesh types exist but deprecated)
- `api/infrastructure/v1alpha1/` - HardwareProfile (deprecated version)

### Controllers
- `internal/controller/components/` - Component controllers (dashboard, kserve, workbenches, etc.)
- `internal/controller/services/` - Service controllers (monitoring, auth, gateway)
- `internal/controller/datasciencecluster/` - DSC controller
- `internal/controller/dscinitialization/` - DSCI controller

### Core Framework
- `pkg/controller/reconciler/` - Generic reconciler framework
- `pkg/controller/actions/` - Reusable reconciliation actions
- `pkg/controller/types/` - ReconciliationRequest and core types
- `pkg/cluster/` - Platform detection and configuration
- `pkg/deploy/` - Manifest deployment utilities
- ~~`pkg/feature/`~~ - Feature API removed in 3.0 (directory empty)

### Supporting Code
- `pkg/plugins/` - Kustomize plugins for manifest transformation (annotation, label, namespace, remover)
- `pkg/conversion/` - API version conversion webhooks
- `pkg/metadata/` - Labels and annotations constants
- `pkg/upgrade/` - Version migration logic
- `pkg/initialinstall/` - Component initialization
- `pkg/resources/` - CRD and RBAC resource management
- `pkg/rules/` - Prometheus rule validation
- `pkg/webhook/` - Webhook utilities
- `pkg/logger/` - Logging utilities
- `pkg/common/` - Common utilities
- `pkg/utils/` - General utilities:
  - `test/` - Test helpers (testf, matchers, jq, fakeclient, envt)
  - `flags/` - CLI flag handling
  - `template/` - Template utilities
- `internal/webhook/` - Admission webhooks

### Tests
- `tests/e2e/` - End-to-end test suites (35+ test files)
- `tests/integration/` - Integration tests
- `tests/envtestutil/` - Environment test setup utilities
- `tests/prometheus_unit_tests/` - Prometheus alert unit tests
- `internal/controller/components/<component>/*_test.go` - Unit tests
- `pkg/utils/test/` - Advanced test utilities:
  - `testf/` - Custom assertion framework with WithT interface
  - `jq/` - JSON query matching for assertions
  - `matchers/` - Gomega custom matchers
  - `fakeclient/` - Mock Kubernetes client
  - `scheme/` - Test scheme builder

## Testing Guidelines

### Unit Tests
- Use Ginkgo/Gomega framework
- Test files: `*_test.go` alongside source files
- Run specific tests: `ginkgo -r <package-path>`
- Requires `make manifests` before running (needs CRDs in `<config-dir>/crd/bases`)

### E2E Tests
- Located in `tests/e2e/`
- Test structure: `tests/e2e/<component>_test.go`
- Configure via environment variables (see README.md for full list):
  - `E2E_TEST_OPERATOR_NAMESPACE`
  - `E2E_TEST_APPLICATIONS_NAMESPACE`
  - `E2E_TEST_COMPONENT` - Space or comma-separated list
  - `E2E_TEST_DELETION_POLICY` - `always`, `on-failure`, or `never`

**Adding tests for new components:**
1. Create `tests/e2e/<component>_test.go`
2. Update `setupDSCInstance()` in `tests/e2e/helper_test.go`
3. Update `newDSC()` in `internal/webhook/webhook_suite_test.go`
4. Add to `componentsTestSuites` map in `tests/e2e/controller_test.go`

### Prometheus Alert Tests
- Rules: `internal/controller/components/<component>/monitoring/<component>-prometheusrules.tmpl.yaml`
- Tests: `internal/controller/components/<component>/monitoring/<component>-alerting.unit-tests.yaml`
- Run: `make test-alerts`
- Check coverage: `make check-prometheus-alert-unit-tests`

## Important Development Notes

### Server-Side Apply (SSA)
- Default resource deployment mode
- Allows field-level ownership (multiple controllers can manage same resource)
- Configured in deploy action: `deploy.NewAction().WithMode(deploy.ModeServerSideApply)`

### Garbage Collection
- **CRITICAL:** GC action must always be the last action before `.Build()`
- Removes resources not in current manifest set
- Uses owner references and labels for tracking

### FeatureTrackers (LEGACY - Removed in 3.0)
- **Status:** NOT actively used - exists for backwards compatibility only
- **CRD:** `api/features/v1/` - CRD definition kept for cleanup
- **Implementation:** `pkg/feature/` - Contains ZERO Go files (removed)
- **Purpose (historical):** Managed cross-namespace resources with cluster-scoped ownership
- **Current usage:**
  - DSCI controller DELETES old ServiceMesh FeatureTrackers during reconciliation
  - Deploy actions recognize FeatureTracker as legacy owner reference (for cleanup)
  - E2E tests verify NO FeatureTrackers exist (see `tests/e2e/kserve_test.go:116`)
- **Why removed:** Service Mesh removal in 3.0 eliminated need for cross-namespace resource management
- **Migration:** Current architecture uses direct owner references within namespaces

### Resource Ownership
- Component CRs owned by DataScienceCluster
- Deployed resources owned by component CRs via direct owner references
- Uses controller-runtime's `SetControllerReference()`
- Cross-namespace resources (e.g., monitoring) use Service CRs as owners

### Working with Custom Application Namespace
When using non-default application namespace:
1. Create namespace before operator installation
2. Add label: `opendatahub.io/application-namespace: true`
3. Set `.spec.applicationsNamespace` in DSCI CR
4. For e2e tests, export `E2E_TEST_APPLICATIONS_NAMESPACE`

### Immutable Fields
Some component fields are immutable after first creation:
- `Workbenches.workbenchNamespace` - Cannot be changed once set
- Enforced via admission webhooks

### API Version Strategy
- **Storage version**: `v2` (in `api/*/v2/`) - Actual version stored in etcd
- **Served version**: `v1` (in `api/*/v1/`) - For backwards compatibility with older clients
- **Conversion webhooks**: Automatic translation between v1 ↔ v2 (see `pkg/conversion/`)
- **New deployments**: Should use v2 API (v1 is legacy)
- **When adding fields**: Add to both versions and update conversion logic

## Debugging Tips

### Local Development Workflow
```bash
# 1. Run operator locally without webhooks
make run-nowebhook

# 2. In another terminal, run e2e tests with debugging flags
make e2e-test \
  -e E2E_TEST_OPERATOR_CONTROLLER=false \
  -e E2E_TEST_WEBHOOK=false \
  -e E2E_TEST_COMPONENT=dashboard \
  -e E2E_TEST_DELETION_POLICY=never
```

### Checking Component Status
```bash
# View DSCI status
oc get dsci default-dsci -o yaml

# View DSC status
oc get dsc default-dsc -o yaml

# View specific component CR
oc get dashboard default-dashboard -o yaml

# View service CRs
oc get monitoring -A
oc get auth -A
```

### Common Issues

**Problem:** CRDs not found during unit tests
**Solution:** Run `make manifests` to generate CRDs in `<config-dir>/crd/bases`

**Problem:** Webhook errors during local development
**Solution:** Use `make run-nowebhook` or set `-tags nowebhook` in Go build

**Problem:** E2E tests fail with "DSCI not found"
**Solution:** Ensure DSCI CR exists before creating DSC

**Problem:** Component stays in "Progressing" state
**Solution:** Check deployment status: `oc get deployments -n <namespace>`. Check operator logs for errors.

## Build System Notes

### Makefile Variables
- `IMG` - Operator image (default: `quay.io/opendatahub/opendatahub-operator:latest`)
- `ODH_PLATFORM_TYPE` - `OpenDataHub` or `rhoai` (default: `OpenDataHub`)
- `VERSION` - Operator version (default: `3.3.0`)
- `OPERATOR_NAMESPACE` - Operator deployment namespace
- `APPLICATIONS_NAMESPACE` - Component deployment namespace
- `USE_LOCAL` - Use local manifests instead of fetching (`true`/`false`)
- `IMAGE_BUILDER` - Container builder (default: `podman`)
- `BUNDLE_IMGS` - Comma-separated bundle images for catalog building

### Configuration Override
Create `local.mk` in repository root to override Makefile variables:
```makefile
IMAGE_TAG_BASE = quay.io/<your-org>/opendatahub-operator
OPERATOR_NAMESPACE = my-operator-namespace
```

### Go Version
- Required: **Go 1.25.0** (see `go.mod`)
- Operator SDK: **v1.39.2**
- Controller Runtime: **v0.20.4**
- Kubernetes API: **v0.32.4**

## Additional Resources

### Core Documentation
- [Mental Map](MENTAL_MAP.md) - Visual conceptual map of the codebase architecture
- [Runtime Deep Dive](RUNTIME_DEEP_DIVE.md) - Operator startup, race conditions, debugging guide
- [Component Integration Guide](docs/COMPONENT_INTEGRATION.md) - Detailed steps for adding new components
- [API Overview](docs/api-overview.md) - Complete API reference (132KB comprehensive guide)
- [Design Document](docs/DESIGN.md) - Architectural design decisions
- [Reading Guide](docs/READING_GUIDE.md) - Line-by-line codebase reading guide (500+ lines)
- [Troubleshooting](docs/troubleshooting.md) - Common issues and solutions

### Release & Testing
- [Release Workflow](docs/release-workflow-guide.md) - Release process
- [Upgrade Testing](docs/upgrade-testing.md) - Testing operator upgrades
- [Integration Testing](docs/integration-testing.md) - Integration test guide
- [E2E Update Guidelines](docs/e2e-update-requirement-guidelines.md) - E2E test guidelines

### Deployment & Operations
- [OLM Deployment](docs/OLMDeployment.md) - Operator Lifecycle Manager deployment
- [Automated Manifest Updates](docs/AUTOMATED_MANIFEST_UPDATES.md) - Manifest update automation

### Advanced Topics
- [Accelerator Metrics](docs/ACCELERATOR_METRICS.md) - GPU/accelerator metric collection
- [Namespace Restricted Metrics](docs/NAMESPACE_RESTRICTED_METRICS.md) - Metric namespace handling
