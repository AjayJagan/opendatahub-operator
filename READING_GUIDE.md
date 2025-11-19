# Line-by-Line Reading Guide for opendatahub-operator

This guide provides a structured path to understand the entire codebase from the ground up.

## Reading Order & Key Concepts

### Phase 1: Foundation (Start Here)

#### 1.1 Entry Point & Initialization
**File:** [cmd/main.go](cmd/main.go)

**What to read:**
- Lines 88-105: Component/service imports - notice the blank imports `_`
- Lines 112-144: `init()` function - scheme registration
- Lines 183-256: `main()` function - operator startup sequence
- Lines 377-401: Controller setup
- Lines 552-575: Registry iteration pattern

**Key concepts:**
- How Go's `init()` functions enable the registry pattern
- Controller-runtime manager setup
- Cache configuration for performance optimization
- Webhook registration

**Questions to answer:**
1. What happens when a component package is imported with `_`?
2. How does the operator determine if it's running ODH vs RHOAI?
3. What's the purpose of the cache configuration (lines 273-325)?

---

#### 1.2 Platform Detection
**Files:**
- [pkg/cluster/const.go](pkg/cluster/const.go) - Platform constants
- [pkg/cluster/cluster_config.go](pkg/cluster/cluster_config.go) - Cluster configuration

**What to read:**
- Platform type definitions (`OpenDataHub`, `SelfManagedRhoai`, `ManagedRhoai`)
- `Init()` function - how platform is detected
- Application namespace resolution

**Key concepts:**
- Build tags vs runtime platform detection
- Cluster-wide singleton configuration
- Namespace management strategy

---

### Phase 2: API Layer

#### 2.1 Common Types
**File:** [api/common/status.go](api/common/status.go)

**What to read:**
- `Status` struct - the base status embedded in all CRs
- `Condition` type - Kubernetes condition pattern
- `ManagementState` - how components are enabled/disabled

**Key concepts:**
- Kubernetes status subresource pattern
- Condition types: Ready, Progressing, Degraded
- Status management helpers

---

#### 2.2 Core CRDs
Read in this order:

**a) DSCInitialization**
- [api/dscinitialization/v1/dscinitialization_types.go](api/dscinitialization/v1/dscinitialization_types.go)

**Focus on:**
- `DSCInitializationSpec` - what gets configured cluster-wide
- `ApplicationsNamespace` field - where components deploy
- `ServiceMesh` configuration
- `Monitoring` configuration

**Questions:**
1. Why is there both v1 and v2? (Check kubebuilder markers)
2. What's the purpose of `TrustedCABundle`?

**b) DataScienceCluster**
- [api/datasciencecluster/v1/datasciencecluster_types.go](api/datasciencecluster/v1/datasciencecluster_types.go)

**Focus on:**
- `Components` struct - all available components
- Each component's `DSC<Component>` type (e.g., `DSCDashboard`)
- `ComponentsStatus` - status aggregation pattern

**Questions:**
1. How does a component's spec in DSC differ from its internal CR spec?
2. What's the relationship between `DSCDashboard` and `Dashboard` CR?

**c) Component CRs**
Pick one to study deeply (Dashboard is simplest):
- [api/components/v1alpha1/dashboard_types.go](api/components/v1alpha1/dashboard_types.go)

**Focus on:**
- `DashboardSpec` and `DashboardCommonSpec`
- Kubebuilder validation markers
- Status structure
- Singleton enforcement (`XValidation` marker)

---

### Phase 3: Controller Architecture

#### 3.1 Registry Pattern
**Files:**
- [internal/controller/components/registry/registry.go](internal/controller/components/registry/registry.go)
- [internal/controller/services/registry/registry.go](internal/controller/services/registry/registry.go)

**What to read:**
- `ComponentHandler` interface
- `Add()` function - how components register themselves
- `ForEach()` - iteration pattern

**Key concepts:**
- Interface-based extensibility
- Self-registration via `init()`
- Type-safe iteration

---

#### 3.2 Generic Reconciler Framework
**Files:**
- [pkg/controller/reconciler/reconciler.go](pkg/controller/reconciler/reconciler.go)
- [pkg/controller/types/types.go](pkg/controller/types/types.go)

**What to read:**
- `ReconcilerFor()` builder pattern
- `ReconciliationRequest` type
- Action signature: `func(ctx, *ReconciliationRequest) error`
- How `.Build()` connects everything

**Key concepts:**
- Go generics for type-safe reconciliation
- Action pipeline pattern
- Builder fluent API

---

#### 3.3 Actions (Reconciliation Steps)
Read these action implementations:

**a) Initialize Action**
Look at any component's initialize action, e.g.:
- [internal/controller/components/dashboard/dashboard_controller_actions.go](internal/controller/components/dashboard/dashboard_controller_actions.go)

**Focus on:**
- How manifest paths are registered
- Platform-specific logic

**b) Kustomize Rendering**
- [pkg/controller/actions/render/kustomize/kustomize.go](pkg/controller/actions/render/kustomize/kustomize.go)

**Focus on:**
- How Kustomize overlays are selected based on platform
- Parameter substitution
- Caching mechanism

**c) Deploy Action**
- [pkg/controller/actions/deploy/deploy.go](pkg/controller/actions/deploy/deploy.go)

**Focus on:**
- Server-Side Apply (SSA) vs Patch mode
- Owner reference management
- Resource filtering

**d) Garbage Collection**
- [pkg/controller/actions/gc/gc.go](pkg/controller/actions/gc/gc.go)

**Focus on:**
- How orphaned resources are identified
- Label-based tracking
- Why it must be last in the action pipeline

**e) Status Actions**
- [pkg/controller/actions/status/deployments/deployments.go](pkg/controller/actions/status/deployments/deployments.go)

**Focus on:**
- How deployment readiness is checked
- Status condition updates

---

### Phase 4: Core Controllers

#### 4.1 DSCInitialization Controller
**File:** [internal/controller/dscinitialization/dscinitialization_controller.go](internal/controller/dscinitialization/dscinitialization_controller.go)

**Read order:**
1. `Reconcile()` method - main reconciliation loop
2. Helper functions for:
   - Application namespace creation
   - Service mesh configuration
   - Monitoring setup
   - Auth creation

**Key concepts:**
- Cluster-wide singleton enforcement
- Service CR creation
- Network policy setup
- Platform-specific behavior

---

#### 4.2 DataScienceCluster Controller
**File:** [internal/controller/datasciencecluster/datasciencecluster_controller.go](internal/controller/datasciencecluster/datasciencecluster_controller.go)

**Read order:**
1. `NewDataScienceClusterReconciler()` - controller builder
2. Actions defined in the pipeline
3. [internal/controller/datasciencecluster/datasciencecluster_actions.go](internal/controller/datasciencecluster/datasciencecluster_actions.go)
   - `provisionComponents` action - creates component CRs
   - `updateStatus` action - aggregates status from components

**Key concepts:**
- Component CR lifecycle (create/update/delete based on `ManagementState`)
- Status aggregation pattern
- Precondition checking (DSCI must exist)

---

#### 4.3 Component Controller (Deep Dive)
Pick one component to study completely. **Dashboard** is recommended as it's relatively simple.

**Files:**
- [internal/controller/components/dashboard/dashboard.go](internal/controller/components/dashboard/dashboard.go) - Component handler implementation
- [internal/controller/components/dashboard/dashboard_controller.go](internal/controller/components/dashboard/dashboard_controller.go) - Reconciler setup
- [internal/controller/components/dashboard/dashboard_controller_actions.go](internal/controller/components/dashboard/dashboard_controller_actions.go) - Custom actions
- [internal/controller/components/dashboard/dashboard_support.go](internal/controller/components/dashboard/dashboard_support.go) - Helper functions

**Reading order:**
1. **dashboard.go**:
   - `componentHandler` struct and interface methods
   - `init()` - self-registration
   - `Init()` - manifest path setup and image parameter substitution
   - `NewCRObject()` - how DSC spec is converted to Dashboard CR

2. **dashboard_controller.go**:
   - `NewComponentReconciler()` - action pipeline definition
   - Notice the order: initialize → kustomize → deploy → status → gc

3. **dashboard_controller_actions.go**:
   - `initialize` - manifest registration
   - `setKustomizedParams` - dynamic parameter computation
   - `updateStatus` - status update logic

4. **dashboard_support.go**:
   - Image parameter mapping
   - Platform-specific utilities

**Questions to answer:**
1. How does the Dashboard controller know which manifests to deploy?
2. What's the flow from DSC spec → Dashboard CR → deployed resources?
3. How are dashboard images customized?

---

### Phase 5: Advanced Topics

#### 5.1 Feature API (Cross-Namespace Resources)
**Files:**
- [pkg/feature/builder.go](pkg/feature/builder.go)
- [pkg/feature/feature.go](pkg/feature/feature.go)
- [api/features/v1/featuretracker_types.go](api/features/v1/featuretracker_types.go)

**Focus on:**
- Why FeatureTracker exists (ownership problem for cross-namespace resources)
- Builder pattern for feature definition
- Resource cleanup mechanism

**Use case example:**
Look at how monitoring uses it:
- [internal/controller/services/monitoring/monitoring_controller.go](internal/controller/services/monitoring/monitoring_controller.go)

---

#### 5.2 Manifest Management
**Files:**
- [pkg/deploy/deploy.go](pkg/deploy/deploy.go) - Core deployment logic
- [get_all_manifests.sh](get_all_manifests.sh) - Manifest fetching script

**Focus on:**
- `ApplyParams()` - environment variable substitution in `params.env`
- Kustomize integration
- Manifest source customization

**Example manifest structure to explore:**
```
opt/manifests/dashboard/
├── base/                    # Look at a few YAMLs here
├── overlays/
│   ├── odh/kustomization.yaml       # Compare these
│   └── rhoai/onprem/kustomization.yaml
└── params.env               # Image references
```

---

#### 5.3 Webhooks
**Files:**
- [internal/webhook/webhooks.go](internal/webhook/webhooks.go) - Registration
- [internal/webhook/hardwareprofile_webhook.go](internal/webhook/hardwareprofile_webhook.go) - Example webhook

**Focus on:**
- Admission webhook pattern (ValidatingWebhook, MutatingWebhook)
- Conversion webhook (for API versioning)
- Webhook registration mechanism

---

#### 5.4 Upgrade & Migration
**Files:**
- [pkg/upgrade/upgrade.go](pkg/upgrade/upgrade.go)
- [pkg/upgrade/uninstall.go](pkg/upgrade/uninstall.go)

**Focus on:**
- Version detection
- Resource cleanup from old versions
- Default CR creation logic

---

### Phase 6: Specific Components (As Needed)

Once you understand Dashboard, you can explore other components:

**Simple components (good for learning):**
- [internal/controller/components/ray/](internal/controller/components/ray/) - Simple controller
- [internal/controller/components/trustyai/](internal/controller/components/trustyai/) - Basic pattern

**Complex components (advanced reading):**
- [internal/controller/components/kserve/](internal/controller/components/kserve/) - Has dependencies, service mesh integration
- [internal/controller/components/datasciencepipelines/](internal/controller/components/datasciencepipelines/) - Conditional sub-components

---

## Reading Tips

### 1. Use Your IDE's "Go to Definition" Feature
- When you see an interface method call, jump to its implementation
- Follow the chain: main → registry → component handler → reconciler → actions

### 2. Draw Diagrams
As you read, sketch:
- Component relationships (DSCInitialization → DataScienceCluster → Components)
- Action pipeline flow
- Registry pattern structure

### 3. Run Tests While Reading
```bash
# Read the test, then run it to see behavior
cd internal/controller/components/dashboard
ginkgo -v

# Read e2e test to understand expected behavior
cat tests/e2e/dashboard_test.go
```

### 4. Check Git Blame for Context
If something seems odd, check when/why it was added:
```bash
git blame <file> -L <start-line>,<end-line>
git show <commit-hash>
```

### 5. Key Files to Keep Open
Have these open in tabs for reference:
- [api/common/status.go](api/common/status.go) - Status pattern
- [pkg/controller/types/types.go](pkg/controller/types/types.go) - ReconciliationRequest
- [pkg/metadata/labels.go](pkg/metadata/labels.go) - Label constants
- [pkg/cluster/const.go](pkg/cluster/const.go) - Platform constants

---

## Checkpoints: Can You Answer These?

### After Phase 1:
- [ ] How does the operator start up?
- [ ] What's the difference between ODH and RHOAI builds?
- [ ] How do components register themselves?

### After Phase 2:
- [ ] What's the relationship between DSCI and DSC?
- [ ] How is `ManagementState` used?
- [ ] What's stored in each CR's status?

### After Phase 3:
- [ ] What's an action in this codebase?
- [ ] How does the generic reconciler work?
- [ ] Why must GC be the last action?

### After Phase 4:
- [ ] Walk through the full flow: User creates DSC → Dashboard deployed
- [ ] How does status propagate from Deployment → Dashboard CR → DSC?
- [ ] What happens when a component's `ManagementState` changes to `Removed`?

### After Phase 5:
- [ ] How do manifests get customized per platform?
- [ ] What's the purpose of FeatureTracker?
- [ ] How does the operator handle upgrades?

---

## Common Patterns to Recognize

### 1. Registry Pattern
```go
// Component self-registers via init()
func init() {
    cr.Add(&componentHandler{})
}

// Later, main.go iterates
cr.ForEach(func(ch ComponentHandler) error {
    return ch.NewComponentReconciler(ctx, mgr)
})
```

### 2. Builder Pattern
```go
reconciler.ReconcilerFor(mgr, &Dashboard{}).
    Owns(&Deployment{}).
    WithAction(initialize).
    WithAction(deploy.NewAction()).
    Build(ctx)
```

### 3. Action Pattern
```go
func myAction(ctx context.Context, rr *types.ReconciliationRequest) error {
    // Access the CR being reconciled
    dashboard := rr.Instance.(*Dashboard)

    // Access client
    client := rr.Client

    // Do reconciliation work
    return nil
}
```

### 4. Status Management
```go
// Components typically do:
conditions.SetCompleted(instance, "Ready", "Dashboard is ready")
conditions.SetInProgress(instance, "Progressing", "Deploying dashboard")
conditions.SetError(instance, "Degraded", err)
```

### 5. Platform Switching
```go
//go:build !rhoai
// ODH-specific code

//go:build rhoai
// RHOAI-specific code

// Or runtime:
if cluster.GetRelease().Name == cluster.ManagedRhoai {
    // managed-specific logic
}
```

---

## Next Steps

After completing this reading guide:

1. **Try modifying something small:**
   - Add a log statement to an action
   - Change a default value in a CRD
   - Add a simple validation to a webhook

2. **Read the integration doc:**
   - [docs/COMPONENT_INTEGRATION.md](docs/COMPONENT_INTEGRATION.md)
   - Try adding a fake component using `make new-component COMPONENT=test`

3. **Understand a specific feature:**
   - Pick something from DSC spec that interests you
   - Trace it from API → controller → deployed resource

4. **Read test code:**
   - Tests often reveal expected behavior better than code
   - Start with unit tests, move to integration, then e2e

---

## Getting Help

If stuck on a concept:
1. Check `docs/` directory for design docs
2. Look at git history: `git log --follow <file>`
3. Find similar code: `git grep -n "pattern"`
4. Read tests: `find . -name "*_test.go" -exec grep -l "concept" {} \;`

Good luck with your deep dive! 🚀
