# Mental Map: OpenDataHub Operator Repository

This document provides a visual and conceptual map of the opendatahub-operator repository to help you understand how everything connects together.

## 🗺️ High-Level Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      OpenDataHub/RHOAI Operator                         │
│                                                                           │
│  ┌────────────────────┐         ┌──────────────────────────────────┐   │
│  │   User Creates:    │         │   Operator Watches & Reconciles:  │   │
│  │                    │         │                                    │   │
│  │  DSCInitialization │────────▶│  DSCI Controller                  │   │
│  │  (singleton)       │         │    ├─ Setup Namespaces            │   │
│  │                    │         │    ├─ Create Service CRs          │   │
│  │                    │         │    └─ Configure Service Mesh      │   │
│  └────────────────────┘         └──────────────────────────────────┘   │
│           │                                    │                         │
│           │ requires DSCI first               ▼                         │
│           │                     ┌─────────────────────────────┐         │
│  ┌────────▼───────────┐         │  Service Controllers:       │         │
│  │  DataScienceCluster│────────▶│   ├─ Monitoring            │         │
│  │  (singleton)       │         │   ├─ Auth                  │         │
│  │                    │         │   ├─ Gateway               │         │
│  │  Spec:             │         │   └─ Setup                 │         │
│  │   .components:     │         └─────────────────────────────┘         │
│  │     dashboard:     │                                                  │
│  │     kserve:        │         ┌──────────────────────────────────┐   │
│  │     workbenches:   │────────▶│  DSC Controller                  │   │
│  │     ...            │         │    └─ Creates Component CRs      │   │
│  └────────────────────┘         └──────────────────────────────────┘   │
│                                              │                           │
│                                              ▼                           │
│                                  ┌──────────────────────────────┐       │
│                                  │  Component CRs (18 types):   │       │
│                                  │   - Dashboard                │       │
│                                  │   - Kserve                   │       │
│                                  │   - Workbenches              │       │
│                                  │   - DataSciencePipelines     │       │
│                                  │   - Ray, SparkOperator       │       │
│                                  │   - MLflowOperator, Feast    │       │
│                                  │   - ModelRegistry, etc.      │       │
│                                  └──────────────────────────────┘       │
│                                              │                           │
│                                              ▼                           │
│                                  ┌──────────────────────────────┐       │
│                                  │ Component Controllers        │       │
│                                  │  (One per component)         │       │
│                                  │                              │       │
│                                  │  Action Pipeline:            │       │
│                                  │   1. Initialize              │       │
│                                  │   2. Render (Kustomize)      │       │
│                                  │   3. Deploy (SSA)            │       │
│                                  │   4. Status Update           │       │
│                                  │   5. Garbage Collection      │       │
│                                  └──────────────────────────────┘       │
│                                              │                           │
│                                              ▼                           │
│                                  ┌──────────────────────────────┐       │
│                                  │  Kubernetes Resources:       │       │
│                                  │   - Deployments              │       │
│                                  │   - Services                 │       │
│                                  │   - ConfigMaps               │       │
│                                  │   - Routes/Ingresses         │       │
│                                  │   - RBAC                     │       │
│                                  └──────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────────┘
```

## 📁 Directory Structure Mental Model

Think of the repository as organized into these functional zones:

### Zone 1: API Definitions (`api/`)
**Purpose**: Define "what" the operator manages

```
api/
├── common/                    # Shared types across all APIs
│   ├── Status, Condition      # Status reporting patterns
│   └── Platform detection     # ODH vs RHOAI
│
├── dscinitialization/         # Cluster-wide setup
│   ├── v1/ (storage)          # Actual stored version
│   └── v2/ (served)           # User-facing version
│
├── datasciencecluster/        # Component orchestration
│   ├── v1/ (storage)
│   └── v2/ (served)
│
├── components/v1alpha1/       # Individual components (18 types)
│   ├── dashboard_types.go
│   ├── kserve_types.go
│   └── ...
│
├── services/v1alpha1/         # Platform services
│   ├── monitoring_types.go
│   ├── auth_types.go
│   ├── gateway_types.go
│   └── servicemesh_types.go
│
├── infrastructure/v1/         # Infrastructure resources
│   ├── hardwareprofile_types.go
│   ├── certificate_types.go
│   └── serverless_types.go
│
└── features/v1/               # Cross-namespace resource ownership
    └── featuretracker_types.go
```

**Key Insight**: Each `_types.go` file defines:
1. `Spec` - What the user wants
2. `Status` - Current state of the resource
3. `DSC<Component>` - Wrapper type for DataScienceCluster integration

### Zone 2: Controllers (`internal/controller/`)
**Purpose**: Define "how" the operator manages resources

```
internal/controller/
├── dscinitialization/         # Creates namespaces, services
├── datasciencecluster/        # Orchestrates components
├── components/                # 18 component controllers
│   ├── dashboard/
│   │   ├── dashboard_controller.go
│   │   ├── dashboard_controller_actions.go  # Action pipeline
│   │   └── monitoring/                      # Prometheus rules
│   ├── kserve/
│   ├── sparkoperator/        # Recently added!
│   ├── mlflowoperator/       # Recently added!
│   └── ...
├── services/                  # Service controllers
│   ├── monitoring/
│   ├── auth/
│   ├── gateway/
│   └── setup/
└── components/registry/       # Component self-registration
```

**Key Insight**: Each component controller follows this pattern:
- `<component>_controller.go` - Reconciler setup
- `<component>_controller_actions.go` - Action pipeline definition
- `monitoring/` - Prometheus alerts and rules

### Zone 3: Framework & Utilities (`pkg/`)
**Purpose**: Reusable building blocks

```
pkg/
├── controller/                # Generic reconciliation framework
│   ├── reconciler/            # ⭐ Type-safe generic reconciler
│   │   └── Reconciler[T]      # Uses Go generics!
│   ├── actions/               # ⭐ Composable action library
│   │   ├── deploy/            # Server-Side Apply deployment
│   │   ├── render/            # Kustomize rendering
│   │   ├── gc/                # Garbage collection
│   │   ├── status/            # Status updates
│   │   ├── dependency/        # External operator checks
│   │   ├── sanitycheck/       # Validation
│   │   └── cacher/            # Resource caching
│   ├── types/                 # ReconciliationRequest
│   ├── conditions/            # Condition management
│   └── predicates/            # Event filtering
│
├── cluster/                   # Platform detection & config
│   ├── Platform type (ODH/RHOAI/Managed)
│   └── Resource management
│
├── feature/                   # ⭐ Cross-namespace resource builder
│   └── Builder pattern for multi-namespace resources
│
├── deploy/                    # Manifest deployment
│   ├── Server-Side Apply (SSA)
│   ├── Patch mode
│   └── Environment variable substitution
│
├── manifests/                 # Manifest loading
├── plugins/                   # Kustomize plugins
├── metadata/                  # Labels & annotations
├── upgrade/                   # Version migrations
├── conversion/                # API version conversion
│
└── utils/                     # General utilities
    ├── test/                  # ⭐ Advanced test utilities
    │   ├── testf/             # Custom assertions
    │   ├── jq/                # JSON query matching
    │   ├── matchers/          # Gomega matchers
    │   ├── fakeclient/        # Mock K8s client
    │   └── scheme/            # Test schemes
    ├── flags/
    └── template/
```

**Key Insight**:
- `pkg/controller/reconciler/` - The "engine" (generic, reusable)
- `pkg/controller/actions/` - The "building blocks" (composable)
- `internal/controller/components/` - The "implementations" (specific)

### Zone 4: Manifests (`opt/manifests/`)
**Purpose**: Kubernetes YAML templates for components

```
opt/manifests/
├── dashboard/
│   ├── base/                  # Base resources
│   ├── overlays/
│   │   ├── odh/               # ODH-specific
│   │   └── rhoai/
│   │       ├── onprem/        # Self-managed RHOAI
│   │       └── addon/         # Managed RHOAI (cloud)
│   └── params.env             # Image references
├── kserve/
├── workbenches/
└── ... (one per component)
```

**Key Insight**: This is embedded in the operator image at build time!

### Zone 5: Tests (`tests/`)
**Purpose**: Validation and verification

```
tests/
├── e2e/                       # 35+ end-to-end tests
│   ├── dsc_controller_test.go
│   ├── dsci_controller_test.go
│   ├── dashboard_test.go
│   ├── kserve_test.go
│   └── ... (one per component)
├── integration/               # Integration tests
├── envtestutil/              # Test environment setup
└── prometheus_unit_tests/    # Alert rule tests
```

## 🔄 Data Flow: From User Intent to Running Workload

```
Step 1: User creates DSCInitialization
   ├─ yaml: DSCInitialization CR with platform config
   │
   ▼
Step 2: DSCI Controller reconciles
   ├─ Creates/updates namespaces (opendatahub or redhat-ods-applications)
   ├─ Creates Monitoring CR → Monitoring Controller → Prometheus/Tempo
   ├─ Creates Auth CR → Auth Controller → kube-auth-proxy
   ├─ Creates Gateway CR → Gateway Controller → Ingress/Routes
   └─ Creates ServiceMesh CR → Service mesh setup
   │
   ▼
Step 3: User creates DataScienceCluster
   ├─ yaml: DSC with .spec.components.<component>.managementState = Managed
   │
   ▼
Step 4: DSC Controller reconciles
   ├─ Validates DSCI exists
   ├─ For each component with managementState=Managed:
   │   └─ Creates Component CR (e.g., Dashboard CR)
   └─ Aggregates status from all Component CRs
   │
   ▼
Step 5: Component Controller reconciles (e.g., Dashboard)
   │
   ├─ Action 1: Initialize
   │   └─ Set manifest paths based on platform (ODH/RHOAI)
   │
   ├─ Action 2: Render (Kustomize)
   │   ├─ Load base manifests from opt/manifests/dashboard/base/
   │   ├─ Apply overlay for platform (odh/ or rhoai/onprem/ or rhoai/addon/)
   │   ├─ Substitute image refs from params.env
   │   └─ Output: Kubernetes resources (YAML)
   │
   ├─ Action 3: Deploy (Server-Side Apply)
   │   ├─ For each resource:
   │   │   ├─ Set owner reference (Component CR)
   │   │   ├─ Apply using Server-Side Apply (field-level ownership)
   │   │   └─ K8s creates/updates: Deployment, Service, ConfigMap, etc.
   │   └─ Resources now running in cluster
   │
   ├─ Action 4: Status Update
   │   ├─ Check deployment readiness
   │   ├─ Update Component CR status (Ready/Progressing/Degraded)
   │   └─ DSC Controller aggregates this to DSC status
   │
   └─ Action 5: Garbage Collection
       ├─ Find resources with owner ref but not in current manifest set
       └─ Delete orphaned resources (cleanup)
   │
   ▼
Step 6: Result
   └─ Workload pods running (e.g., dashboard pods)
   └─ User can access via Route/Ingress
```

## 🧩 Component Interaction Map

### How Components Talk to Each Other

```
┌──────────────┐
│  Dashboard   │─────┐
└──────────────┘     │
                     │
┌──────────────┐     │    All components use:
│  Workbenches │─────┤    ┌────────────────────┐
└──────────────┘     │───▶│  Gateway (Routes)  │
                     │    └────────────────────┘
┌──────────────┐     │           │
│   Kserve     │─────┤           ▼
└──────────────┘     │    ┌────────────────────┐
                     │───▶│  Auth (SSO/OIDC)   │
┌──────────────┐     │    └────────────────────┘
│      Ray     │─────┤           │
└──────────────┘     │           ▼
                     │    ┌────────────────────┐
┌──────────────┐     │───▶│  Monitoring        │
│ DSPipelines  │─────┘    │  (Prometheus)      │
└──────────────┘          └────────────────────┘

All components also use:
  - FeatureTracker for cross-namespace resources
  - HardwareProfile for GPU/accelerator allocation
  - ServiceMesh for network policies
```

### Component Categories

```
┌─────────────────────────────────────────────────────────┐
│                   Component Ecosystem                   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  📊 UI & User Interaction                              │
│  ├─ Dashboard: Main web UI                             │
│  └─ Workbenches: Jupyter notebook environments         │
│                                                         │
│  🤖 Model Lifecycle                                     │
│  ├─ Kserve: Model serving (InferenceService)           │
│  ├─ ModelController: Model lifecycle management        │
│  ├─ ModelRegistry: Model artifact storage              │
│  └─ ModelMeshServing: [deprecated] Legacy serving      │
│                                                         │
│  🎓 Training                                            │
│  ├─ Trainer: ML training workloads                     │
│  └─ TrainingOperator: Distributed training (TFJob, etc)│
│                                                         │
│  🔄 ML Pipeline & Workflow                             │
│  ├─ DataSciencePipelines: Kubeflow Pipelines v2        │
│  └─ MLflowOperator: Experiment tracking [new!]         │
│                                                         │
│  ⚡ Distributed Computing                              │
│  ├─ Ray: Distributed Python framework                  │
│  ├─ SparkOperator: Spark cluster management [new!]     │
│  └─ CodeFlare: [inactive] Code execution               │
│                                                         │
│  🛠️ ML Infrastructure                                 │
│  ├─ Kueue: Job queuing & resource quotas               │
│  ├─ FeastOperator: Feature store                       │
│  └─ TrustyAI: AI explainability & bias detection       │
│                                                         │
│  🦙 AI Frameworks                                       │
│  └─ LlamaStackOperator: LLM inference                  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## 🔧 The Action Pipeline Pattern

Every component controller uses the same action pipeline pattern:

```
┌─────────────────────────────────────────────────────────┐
│          Component Reconciler (Generic Pattern)         │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
        ┌───────────────────────────────┐
        │  reconciler.ReconcilerFor(T)  │  ← Generic reconciler
        │  .For(Component CR)           │
        └───────────────────────────────┘
                        │
                        ▼
        ┌───────────────────────────────┐
        │    .WithAction(action1)       │  ← Build action pipeline
        │    .WithAction(action2)       │
        │    .WithAction(action3)       │
        │    .WithAction(...)           │
        │    .WithAction(gc.NewAction)  │  ← GC must be last!
        └───────────────────────────────┘
                        │
                        ▼
        ┌───────────────────────────────┐
        │         .Build()              │  ← Returns reconciler
        └───────────────────────────────┘

Actions are functions with signature:
  func(ctx context.Context, rr *ReconciliationRequest) error

Each action:
  1. Reads current state from rr.Instance (the Component CR)
  2. Performs operation (render, deploy, check status)
  3. Updates rr as needed
  4. Returns error if failed (stops pipeline)
  5. Returns nil if success (continues to next action)
```

### Common Action Sequences

**Typical Component**:
```
initialize → kustomize.NewAction → deploy.NewAction →
deployments.NewAction → updateStatus → gc.NewAction
```

**Component with Dependencies**:
```
initialize → dependency.NewAction → kustomize.NewAction →
deploy.NewAction → deployments.NewAction → updateStatus → gc.NewAction
```

**Service (like Monitoring)**:
```
initialize → render.NewAction → deploy.NewAction →
status.NewAction → gc.NewAction
```

## 🏗️ Build Tags & Platform Variants

The operator supports 3 platforms using Go build tags:

```
┌─────────────────────────────────────────────────────┐
│           OpenDataHub (ODH) - Upstream              │
│  Build tag: none (default) or //go:build !rhoai    │
│  Namespace: opendatahub                             │
│  Manifests: opt/manifests/<component>/overlays/odh/ │
│  Features: Community features                       │
└─────────────────────────────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        │                               │
        ▼                               ▼
┌──────────────────────┐   ┌────────────────────────┐
│  Self-Managed RHOAI  │   │   Managed RHOAI        │
│  (On-Premises)       │   │   (Cloud/Addon)        │
├──────────────────────┤   ├────────────────────────┤
│ Tag: //go:build rhoai│   │ Tag: //go:build rhoai  │
│ Namespace:           │   │ Namespace:             │
│  redhat-ods-apps     │   │  redhat-ods-apps       │
│ Manifests:           │   │ Manifests:             │
│  rhoai/onprem/       │   │  rhoai/addon/          │
│ Features:            │   │ Features:              │
│  + SegmentIO         │   │  + Managed services    │
│  + Red Hat support   │   │  + Cloud integration   │
└──────────────────────┘   └────────────────────────┘
```

**Platform Detection** (in `pkg/cluster/`):
```go
platform := cluster.GetPlatform()
// Returns: OpenDataHub | SelfManagedRhoai | ManagedRhoai

if platform.IsRhoai() {
    // RHOAI-specific logic
}
```

## 🔀 Reconciliation Request Flow

Understanding the ReconciliationRequest (rr) is key:

```
┌─────────────────────────────────────────────┐
│        ReconciliationRequest               │
├─────────────────────────────────────────────┤
│  Instance: *ComponentCR                    │  ← The CR being reconciled
│  Client: client.Client                     │  ← K8s API client
│  Logger: logr.Logger                       │  ← Logging
│  Context: context.Context                  │  ← Cancellation
│  Platform: Platform                        │  ← ODH/RHOAI detection
│  Manifests: []unstructured.Unstructured    │  ← Rendered resources
│  Resources: map[string]interface{}         │  ← Cached resources
└─────────────────────────────────────────────┘
           │
           │ Passed through action pipeline
           ▼
    ┌──────────┐   ┌──────────┐   ┌──────────┐
    │ Action 1 │──▶│ Action 2 │──▶│ Action 3 │
    └──────────┘   └──────────┘   └──────────┘
    Each action can:
    - Read rr.Instance
    - Modify rr.Manifests
    - Update rr.Resources (cache)
    - Call rr.Client (K8s API)
    - Log via rr.Logger
```

## 🎯 Key Concept Connections

### Ownership Chain

```
User
 └─ Creates DSCInitialization
     └─ Owns Service CRs
         ├─ Monitoring CR
         │   └─ Owns Prometheus, Tempo deployments
         ├─ Auth CR
         │   └─ Owns kube-auth-proxy deployment
         └─ Gateway CR
             └─ Owns Ingress/Route resources

User
 └─ Creates DataScienceCluster
     └─ Owns Component CRs
         ├─ Dashboard CR
         │   └─ Owns Dashboard deployment, service, route
         ├─ Kserve CR
         │   └─ Owns Kserve controller deployment
         └─ Workbenches CR
             └─ Owns Notebook controller deployment

Special: Cross-namespace resources
 └─ FeatureTracker CR (cluster-scoped)
     └─ Owns resources in multiple namespaces
         Example: Monitoring resources in workbench namespace
```

### Status Aggregation

```
Component CR Status
  ├─ Conditions: Ready, Progressing, Degraded
  ├─ Phase: string
  └─ ObservedGeneration: int64
         │
         │ Aggregated by DSC Controller
         ▼
DataScienceCluster Status
  ├─ Conditions: Ready, Progressing, Degraded (from all components)
  ├─ ComponentConditions: map[component]Condition
  └─ Phase: string (overall state)
```

### Manifest Rendering Pipeline

```
params.env (images & env vars)
         │
         ▼
base/kustomization.yaml
         │
         ▼
overlays/{odh|rhoai/onprem|rhoai/addon}/kustomization.yaml
         │
         ▼
Kustomize Plugins (in pkg/plugins/)
  ├─ AddAnnotationPlugin
  ├─ AddLabelPlugin
  ├─ AddNamespacePlugin
  └─ RemoverPlugin
         │
         ▼
Rendered Kubernetes Resources (unstructured)
         │
         ▼
Server-Side Apply (SSA) to K8s API
         │
         ▼
Running Resources in Cluster
```

## 🧪 Testing Mental Model

### Test Hierarchy

```
Unit Tests (internal/controller/*/test.go)
  ├─ Test individual functions
  ├─ Use fake K8s client (pkg/utils/test/fakeclient)
  └─ Fast, isolated

Integration Tests (tests/integration/)
  ├─ Test multiple components together
  ├─ Use envtest (real API server, no cluster)
  └─ Medium speed

E2E Tests (tests/e2e/)
  ├─ Test full operator lifecycle
  ├─ Require real K8s cluster
  ├─ Deploy operator, create CRs, verify workloads
  └─ Slow, comprehensive

Prometheus Alert Tests (tests/prometheus_unit_tests/)
  ├─ Test alert rules fire correctly
  ├─ Use promtool unit test format
  └─ Fast, specialized
```

### Test Utilities

```
pkg/utils/test/
├── testf/          → Custom assertion framework
│   Example: testf.New(t).WithObj(obj).Assert(condition)
│
├── jq/             → JSON query matching
│   Example: jq.Match(".spec.replicas", 3)
│
├── matchers/       → Gomega custom matchers
│   Example: Expect(obj).To(matchers.HaveCondition("Ready"))
│
├── fakeclient/     → Mock K8s client for unit tests
│   Example: client := fakeclient.New(scheme, objects...)
│
└── scheme/         → Test scheme builder
    Example: scheme := scheme.New().WithComponents().Build()
```

## 🚀 Development Workflow Visualization

```
┌─────────────────────────────────────────────────────────┐
│              Developer Workflow                         │
└─────────────────────────────────────────────────────────┘

1. Make code changes
   ├─ API changes? → make generate manifests api-docs
   ├─ New component? → make new-component COMPONENT=foo
   └─ Regular code → just edit

2. Run tests locally
   ├─ Unit tests → make unit-test
   ├─ Lint → make lint (or make lint-fix)
   └─ Format → make fmt

3. Test in cluster
   ├─ Build image → make image-build IMG=quay.io/me/operator:test
   ├─ Push image → make image-push IMG=quay.io/me/operator:test
   └─ Deploy → make deploy IMG=quay.io/me/operator:test

   OR for local development:
   ├─ Run locally → make run (with webhooks)
   └─ Run locally → make run-nowebhook (debugging)

4. E2E testing
   ├─ All components → make e2e-test
   ├─ Specific component → make e2e-test -e E2E_TEST_COMPONENT=dashboard
   └─ Exclude component → make e2e-test -e E2E_TEST_COMPONENT=!ray

5. Create PR
   ├─ CI runs tests automatically
   ├─ E2E images built per PR
   └─ Reviewers check changes
```

## 🎓 Learning Path Recommendation

If you're new to this codebase, follow this learning path:

### Level 1: Understanding the Basics
1. Read [CLAUDE.md](CLAUDE.md) - Project overview
2. Read [docs/DESIGN.md](docs/DESIGN.md) - Architecture decisions
3. Explore `api/` directory - Understand CRD structure
4. Read `pkg/controller/reconciler/` - Core framework

### Level 2: Deep Dive into Controllers
1. Read `internal/controller/datasciencecluster/` - Main orchestrator
2. Pick ONE simple component (e.g., `dashboard/`)
3. Read its `_controller.go` and `_controller_actions.go`
4. Trace the action pipeline: initialize → kustomize → deploy → gc
5. Look at its manifests in `opt/manifests/dashboard/`

### Level 3: Advanced Patterns
1. Study `pkg/feature/` - Cross-namespace resources
2. Study `pkg/controller/actions/deploy/` - Server-Side Apply
3. Study `internal/controller/services/monitoring/` - Service controllers
4. Read [docs/COMPONENT_INTEGRATION.md](docs/COMPONENT_INTEGRATION.md)

### Level 4: Contributing
1. Try `make new-component COMPONENT=test` to see scaffolding
2. Read existing component tests in `tests/e2e/`
3. Read [docs/e2e-update-requirement-guidelines.md](docs/e2e-update-requirement-guidelines.md)
4. Start with small changes, then larger features

## 🔍 Quick Reference: Finding Things

### "Where do I find...?"

**CRD definitions?**
→ `api/<resource-type>/v1/<resource>_types.go`

**How a component is reconciled?**
→ `internal/controller/components/<component>/<component>_controller_actions.go`

**Manifest templates?**
→ `opt/manifests/<component>/`

**Platform-specific code?**
→ Look for `//go:build rhoai` or `//go:build !rhoai` tags

**How to add a new action?**
→ Study `pkg/controller/actions/` and create similar

**How components self-register?**
→ `internal/controller/components/<component>/` - Look for `init()` function

**Test examples?**
→ `tests/e2e/<component>_test.go`

**Build configuration?**
→ `Makefile` and `.github/workflows/`

**Documentation?**
→ `docs/` directory

## 🧭 Navigation Tips

1. **Use tags for platform-specific code**: Search for `//go:build` to find ODH vs RHOAI differences

2. **Follow the imports**: Start from `cmd/main.go` to see what's imported and registered

3. **Use the registry**: `internal/controller/components/registry/` shows all registered components

4. **Check the Makefile**: It shows all available commands and their dependencies

5. **Read the tests**: Tests often document expected behavior better than comments

## 📊 Component Complexity Matrix

```
Simple Components (good starting point):
  ├─ Dashboard - Basic deployment
  └─ TrustyAI - Straightforward setup

Medium Components:
  ├─ Workbenches - Custom namespace handling
  ├─ DataSciencePipelines - Multiple dependencies
  └─ Kueue - External operator integration

Complex Components:
  ├─ Kserve - Service mesh integration, multiple modes
  ├─ Ray - Cluster management, autoscaling
  └─ SparkOperator - Recently added, full-featured

Service Controllers (different pattern):
  ├─ Monitoring - Multi-namespace resource management
  ├─ Gateway - Ingress/route configuration
  └─ Auth - Authentication setup
```

---

## 💡 Key Takeaways

1. **Everything is a CR**: DSCInitialization, DataScienceCluster, Components, Services - all are Custom Resources

2. **Actions are composable**: The action pipeline pattern makes controllers uniform and testable

3. **SSA is default**: Server-Side Apply enables field-level ownership and multi-controller management

4. **Platform matters**: Code often branches based on ODH vs RHOAI using build tags

5. **Ownership is critical**: Owner references drive garbage collection and lifecycle management

6. **Status flows up**: Component status → DSC status → User visibility

7. **Manifests are embedded**: All Kustomize manifests are built into the operator image

8. **Testing is comprehensive**: Unit, integration, E2E, and Prometheus alert tests

9. **Generic reconciler**: The framework in `pkg/controller/reconciler/` makes adding components easier

10. **Registry pattern**: Components self-register, making the system extensible

---

**This mental map should help you navigate the codebase and understand how everything connects!**

For hands-on exploration, start with a simple component like Dashboard and trace its full lifecycle from CRD definition → Controller → Action pipeline → Deployed resources.
