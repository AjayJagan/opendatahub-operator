# Core Architectural Patterns

This document explains the three fundamental design patterns that power the opendatahub-operator architecture. Written for developers new to Go and operator patterns.

**Table of Contents:**
- [Pattern 1: Registry Pattern (Component Self-Registration)](#pattern-1-registry-pattern-component-self-registration)
- [Pattern 2: Actions Pattern (Composable Pipeline)](#pattern-2-actions-pattern-composable-pipeline)
- [Pattern 3: Generic Reconciler (Type Safety with Go Generics)](#pattern-3-generic-reconciler-type-safety-with-go-generics)
- [Putting It All Together](#putting-it-all-together)
- [Kubernetes Clients Explained](#kubernetes-clients-explained)
- [Manager Explained](#manager-explained)

---

## Pattern 1: Registry Pattern (Component Self-Registration)

### What Problem Does It Solve?

With 18 different components (Dashboard, Kserve, Workbenches, etc.), how does the DSC controller discover them without maintaining a hardcoded list?

**Solution:** Components **register themselves** automatically when the program starts using Go's `init()` mechanism.

---

### How It Works

#### Step 1: The Registry (`internal/controller/components/registry/registry.go`)

```go
// Think of this as a phone book for components
type Registry struct {
    handlers []ComponentHandler  // List of registered components
}

// Global singleton instance (created once, shared everywhere)
var r = &Registry{}

// Add a component to the registry
func Add(ch ComponentHandler) {
    r.handlers = append(r.handlers, ch)
}
```

**Go Concept: Package-level variables**
- `var r = &Registry{}` creates a single instance shared across the entire program
- The `&` means "pointer to" - we're storing a memory address, not a copy

---

#### Step 2: The Interface - Contract Every Component Must Fulfill

```go
type ComponentHandler interface {
    GetName() string                            // "dashboard", "kserve", etc.
    Init(platform) error                        // Setup manifests/images
    NewCRObject(dsc) common.PlatformObject      // Create component CR from DSC
    NewComponentReconciler(ctx, mgr) error      // Register controller with manager
    UpdateDSCStatus(ctx, rr) (status, error)    // Update DSC status
    IsEnabled(dsc) bool                         // Check if ManagementState is Managed
}
```

**Go Concept: Interfaces**
- An interface is a **contract** - a list of methods a type must implement
- Any type that implements ALL these methods automatically satisfies the interface
- No explicit "implements" keyword needed (unlike Java)

---

#### Step 3: Dashboard Implements the Interface

**File:** `internal/controller/components/dashboard/dashboard.go`

```go
// 1. Define a type (empty struct, just a namespace)
type componentHandler struct{}

// 2. THE MAGIC: init() runs automatically when package loads
func init() {
    cr.Add(&componentHandler{})  // Register myself!
}

// 3. Implement all interface methods
func (s *componentHandler) GetName() string {
    return "dashboard"
}

func (s *componentHandler) Init(platform common.Platform) error {
    // Apply image parameters to manifests
    mi := defaultManifestInfo(platform)
    return odhdeploy.ApplyParams(mi.String(), "params.env", imagesMap)
}

func (s *componentHandler) NewCRObject(dsc *dscv2.DataScienceCluster) common.PlatformObject {
    // Create a Dashboard CR from DSC spec
    return &componentApi.Dashboard{
        TypeMeta: metav1.TypeMeta{
            Kind:       "Dashboard",
            APIVersion: "components.opendatahub.io/v1alpha1",
        },
        ObjectMeta: metav1.ObjectMeta{
            Name: "default-dashboard",
        },
        Spec: componentApi.DashboardSpec{
            DashboardCommonSpec: dsc.Spec.Components.Dashboard.DashboardCommonSpec,
        },
    }
}

func (s *componentHandler) NewComponentReconciler(ctx context.Context, mgr ctrl.Manager) error {
    // Creates the Dashboard controller (see Pattern 2)
    return setupDashboardController(ctx, mgr)
}

func (s *componentHandler) IsEnabled(dsc *dscv2.DataScienceCluster) bool {
    return dsc.Spec.Components.Dashboard.ManagementState == operatorv1.Managed
}

func (s *componentHandler) UpdateDSCStatus(ctx context.Context, rr *types.ReconciliationRequest) (metav1.ConditionStatus, error) {
    // Fetch Dashboard CR and copy status to DSC
    c := componentApi.Dashboard{}
    c.Name = componentApi.DashboardInstanceName

    if err := rr.Client.Get(ctx, client.ObjectKeyFromObject(&c), &c); err != nil {
        return metav1.ConditionUnknown, err
    }

    dsc := rr.Instance.(*dscv2.DataScienceCluster)
    dsc.Status.Components.Dashboard.DashboardCommonStatus = c.Status.DashboardCommonStatus.DeepCopy()

    return metav1.ConditionTrue, nil
}
```

**Go Concept: init() function**
```go
func init() {
    // This runs AUTOMATICALLY when the package is imported
    // Runs BEFORE main()
    // Perfect for self-registration!
}
```

**Go Concept: Method receivers**
```go
func (s *componentHandler) GetName() string {
    //   ↑ receiver - like "this" in Java/Python
    //     's' is the instance of componentHandler
}
```

---

#### Step 4: How Registration Happens at Runtime

```
Program Starts
     ↓
Go runtime imports all packages
     ↓
internal/controller/components/dashboard imported
     ↓
init() function runs automatically
     ↓
cr.Add(&componentHandler{}) adds Dashboard to registry
     ↓
internal/controller/components/kserve imported
     ↓
init() runs, adds Kserve to registry
     ↓
... 18 components total ...
     ↓
main() starts
     ↓
Registry now has all 18 components!
```

**Import trigger:** The blank imports in `cmd/main.go` force package loading:

```go
import (
    _ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/dashboard"
    _ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/kserve"
    // ... all 18 components
)
```

The `_` means "import for side effects only" (runs init(), doesn't use package directly).

---

#### Step 5: Using the Registry

**DSC Controller:**

```go
// Iterate over ALL registered components
registry.ForEach(func(ch ComponentHandler) error {
    if ch.IsEnabled(dsc) {
        // Create component CR
        obj := ch.NewCRObject(dsc)
        client.Create(ctx, obj)
    }
    return nil
})

// Check if specific component is enabled
if registry.IsComponentEnabled("dashboard", dsc) {
    // Do something
}
```

---

### Why This Pattern Is Powerful

| Benefit | Explanation |
|---------|-------------|
| **No hardcoded list** | DSC controller doesn't need to know about components |
| **Easy to add components** | Just implement interface + init(), import in main.go |
| **Decoupled** | Each component is self-contained in its own package |
| **Discoverable** | All components found automatically via registry |
| **Type-safe** | Interface ensures all components have required methods |

---

## Pattern 2: Actions Pattern (Composable Pipeline)

### What Problem Does It Solve?

Every component needs to do similar things: render manifests, deploy them, check deployment status, garbage collect old resources. How do we avoid duplicating this logic 18 times?

**Solution:** Build a **pipeline of reusable actions** that can be composed differently for each component.

---

### The Action Type

```go
// An action is just a function with this signature
type Fn func(ctx context.Context, rr *ReconciliationRequest) error
//                                     ↑ Contains everything an action needs
```

**Go Concept: Function types**
```go
// In Go, functions are first-class citizens - you can:
// 1. Define a type that IS a function signature
type Fn func(context.Context, *ReconciliationRequest) error

// 2. Store functions in variables
var myAction Fn = func(ctx, rr) error { return nil }

// 3. Pass functions as parameters
func AddAction(action Fn) { ... }

// 4. Return functions from functions (factory pattern)
func NewAction() Fn { ... }
```

---

### ReconciliationRequest - Shared State

**File:** `pkg/controller/types/types.go`

```go
type ReconciliationRequest struct {
    Client     client.Client              // Kubernetes API client (cached)
    Controller Controller                  // Controller interface
    Conditions *conditions.Manager         // Status condition manager
    Instance   common.PlatformObject       // The CR being reconciled (e.g., Dashboard)
    Release    common.Release              // Platform info (ODH/RHOAI)
    Manifests  []ManifestInfo              // Paths to Kustomize manifests
    Templates  []TemplateInfo              // Go templates
    Resources  []unstructured.Unstructured // Rendered Kubernetes resources
    Generated  bool                        // Were resources generated?
}
```

**Think of it as:**
- A **briefcase** passed between actions
- Each action reads/modifies the briefcase contents
- Next action sees previous action's changes
- **Immutable context** (platform, release) + **mutable state** (resources, conditions)

---

### Real Action Examples from Dashboard

#### Action 1: Initialize

**File:** `internal/controller/components/dashboard/dashboard_controller_actions.go:56-60`

```go
func initialize(ctx context.Context, rr *ReconciliationRequest) error {
    // Set the manifest path based on platform
    rr.Manifests = []ManifestInfo{defaultManifestInfo(rr.Release.Name)}
    return nil
}
```

**What it does:** Sets which manifest directory to use based on platform:
- ODH → `opt/manifests/dashboard/odh/`
- RHOAI Self-Managed → `opt/manifests/dashboard/rhoai/onprem/`
- RHOAI Managed → `opt/manifests/dashboard/rhoai/addon/`

---

#### Action 2: Set Kustomize Parameters

```go
func setKustomizedParams(ctx context.Context, rr *ReconciliationRequest) error {
    // Compute dynamic variables (gateway URL, section title)
    extraParamsMap, err := computeKustomizeVariable(ctx, rr.Client, rr.Release.Name)
    if err != nil {
        return fmt.Errorf("failed to set variable for url, section-title etc: %w", err)
    }

    // Apply them to params.env file
    if err := odhdeploy.ApplyParams(rr.Manifests[0].String(), "params.env", nil, extraParamsMap); err != nil {
        return fmt.Errorf("failed to update params.env from %s : %w", rr.Manifests[0].String(), err)
    }
    return nil
}
```

**What it does:** Replaces variables in Kustomize manifests:
- `${dashboard-url}` → `https://gateway.apps.cluster.example.com/`
- `${section-title}` → `"OpenShift Self Managed Services"`

---

#### Action 3: Configure Dependencies

```go
func configureDependencies(ctx context.Context, rr *ReconciliationRequest) error {
    if rr.Release.Name == cluster.OpenDataHub {
        return nil  // ODH doesn't need this
    }

    appNamespace, err := cluster.ApplicationNamespace(ctx, rr.Client)
    if err != nil {
        return err
    }

    // Add an extra Secret resource to deploy
    err = rr.AddResources(&corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "anaconda-ce-access",
            Namespace: appNamespace,
        },
        Type: corev1.SecretTypeOpaque,
    })

    return err
}
```

**What it does:**
- Platform-specific logic (RHOAI only)
- Adds extra resources to `rr.Resources` slice
- Resources will be deployed by later `deploy.NewAction()`

---

#### Action 4-9: Built-in Reusable Actions

These actions are provided by the framework and used by **all components**:

| Action | File | Purpose |
|--------|------|---------|
| `kustomize.NewAction()` | `pkg/controller/actions/render/kustomize/` | Renders Kustomize manifests → `rr.Resources` |
| `deploy.NewAction()` | `pkg/controller/actions/deploy/` | Deploys resources via Server-Side Apply |
| `deployments.NewAction()` | `pkg/controller/actions/status/deployments/` | Checks Deployment readiness → `rr.Conditions` |
| `gc.NewAction()` | `pkg/controller/actions/gc/` | Garbage collects orphaned resources |

---

### The Action Pipeline (Dashboard Controller)

**File:** `internal/controller/components/dashboard/dashboard_controller.go:100-126`

```go
reconciler.ReconcilerFor(mgr, &componentApi.Dashboard{}).
    // ... Watches/Owns setup ...
    WithAction(initialize).                    // 1. Set manifest paths
    WithAction(setKustomizedParams).           // 2. Replace variables
    WithAction(configureDependencies).         // 3. Add extra resources
    WithAction(kustomize.NewAction(            // 4. Render Kustomize manifests
        kustomize.WithLabel(labels.ODH.Component(componentName), labels.True),
        kustomize.WithLabel(labels.K8SCommon.PartOf, componentName),
    )).
    WithAction(deploy.NewAction()).            // 5. Deploy to cluster (SSA)
    WithAction(deployments.NewAction()).       // 6. Check deployment readiness
    WithAction(reconcileHardwareProfiles).     // 7. Migrate HardwareProfiles
    WithAction(updateStatus).                  // 8. Update CR status
    WithAction(gc.NewAction(                   // 9. Garbage collect (MUST BE LAST)
        gc.WithUnremovables(gvk.OdhDashboardConfig),
    )).
    Build(ctx)
```

---

### Execution Flow

```
Reconcile triggered (Dashboard CR created/updated)
     ↓
Create new ReconciliationRequest (empty briefcase)
     ↓
Run initialize(ctx, rr)
   └─> Adds manifest paths to rr.Manifests
       rr.Manifests = ["opt/manifests/dashboard/odh/"]
     ↓
Run setKustomizedParams(ctx, rr)
   └─> Modifies params.env files
       ${dashboard-url} → https://gateway.domain/
     ↓
Run configureDependencies(ctx, rr)
   └─> Adds Secret to rr.Resources
       rr.Resources = [Secret{anaconda-ce-access}]
     ↓
Run kustomize.NewAction()(ctx, rr)
   └─> Renders manifests → rr.Resources
       rr.Resources = [Deployment, Service, Route, ConfigMap, Secret, ...]
     ↓
Run deploy.NewAction()(ctx, rr)
   └─> Applies all rr.Resources to cluster via Server-Side Apply
       kubectl apply --server-side --field-manager=dashboard-controller
     ↓
Run deployments.NewAction()(ctx, rr)
   └─> Checks Deployment status, updates rr.Conditions
       rr.Conditions.MarkTrue("DeploymentsAvailable")
     ↓
Run reconcileHardwareProfiles(ctx, rr)
   └─> Migrates old dashboard HardwareProfiles to infrastructure HardwareProfiles
     ↓
Run updateStatus(ctx, rr)
   └─> Fetches Route, updates Dashboard CR status
       dashboard.Status.URL = "https://dashboard.apps.cluster.example.com"
     ↓
Run gc.NewAction()(ctx, rr)
   └─> Deletes resources not in rr.Resources (orphaned from previous reconcile)
     ↓
Done! Return to reconcile loop
```

---

### Why This Pattern Is Powerful

| Benefit | Explanation |
|---------|-------------|
| **Reusable** | `kustomize.NewAction()` used by all 18 components |
| **Composable** | Mix and match actions as needed per component |
| **Testable** | Test each action in isolation |
| **Readable** | Pipeline shows reconciliation flow at a glance |
| **Flexible** | Insert component-specific actions anywhere |
| **Error handling** | Actions can return `StopError`, `TransientError`, etc. |
| **Ordering** | Pipeline enforces correct execution order (GC must be last) |

---

### Action Error Types

**File:** `pkg/controller/actions/errors/`

```go
// StopError - Stop reconciliation, don't requeue
type StopError struct { ... }

// TransientError - Temporary failure, requeue with backoff
type TransientError struct { ... }

// PermanentError - Fatal error, requeue immediately
type PermanentError struct { ... }
```

Actions can return these to control reconciliation behavior.

---

## Pattern 3: Generic Reconciler (Type Safety with Go Generics)

### What Problem Does It Solve?

Before generics (Go 1.18+), you'd have to:
1. Write `DashboardReconciler`, `KserveReconciler`, `WorkbenchesReconciler`, etc.
2. Each would have duplicate code for fetching CR, running actions, updating status

**Solution:** ONE generic reconciler that works for ANY component type using Go generics!

---

### Go Generics Basics

**Go 1.18+ introduced generics (type parameters):**

```go
// Old way - type specific
func ProcessDashboard(d *Dashboard) { ... }
func ProcessKserve(k *Kserve) { ... }
func ProcessWorkbenches(w *Workbenches) { ... }

// New way - generic
func Process[T any](obj T) { ... }
//          ↑ T is a type parameter (placeholder)
//          any means "T can be any type"

// Usage:
Process[Dashboard](myDashboard)
Process[Kserve](myKserve)
Process[*Workbenches](myWorkbenches)
```

**Type constraints:**

```go
// T must implement PlatformObject interface
func Process[T common.PlatformObject](obj T) { ... }
//            ↑ constraint

// PlatformObject interface:
type PlatformObject interface {
    client.Object        // Kubernetes object (GetName, GetNamespace, etc.)
    GetStatus() *Status  // Status accessor
    // ... more methods
}
```

---

### The Generic Reconciler

**File:** `pkg/controller/reconciler/reconciler.go:71-110`

```go
// NewReconciler creates a reconciler for ANY type T that implements PlatformObject
func NewReconciler[T common.PlatformObject](
    mgr manager.Manager,
    name string,
    object T,                   // Example object (type information)
    opts ...ReconcilerOpt,
) (*Reconciler, error) {

    cc := Reconciler{
        Client:   mgr.GetClient(),
        Scheme:   mgr.GetScheme(),
        Log:      ctrl.Log.WithName("controllers").WithName(name),
        Recorder: mgr.GetEventRecorderFor(name),
        Release:  cluster.GetRelease(),
        name:     name,

        // The MAGIC: Create new instances of T dynamically
        instanceFactory: func() (common.PlatformObject, error) {
            t := reflect.TypeOf(object).Elem()  // Get type of T
            res, ok := reflect.New(t).Interface().(T)  // Create new instance
            if !ok {
                return res, fmt.Errorf("unable to construct instance of %v", t)
            }
            return res, nil
        },

        gvks: make(map[schema.GroupVersionKind]gvkInfo),
    }

    return &cc, nil
}
```

**Go Concept: Reflection**
```go
// reflect package lets you inspect and manipulate types at runtime
t := reflect.TypeOf(object).Elem()     // Get the type (Dashboard, Kserve, etc.)
newInstance := reflect.New(t)          // Create new instance of that type
instance := newInstance.Interface().(T) // Convert to type T
```

---

### Using the Generic Reconciler (Dashboard Example)

**File:** `internal/controller/components/dashboard/dashboard_controller.go`

```go
// Create reconciler for Dashboard type
reconciler.ReconcilerFor(mgr, &componentApi.Dashboard{}).
    //                           ↑ Tells Go: T = *componentApi.Dashboard
    Owns(&corev1.ConfigMap{}).
    Owns(&corev1.Secret{}).
    Owns(&appsv1.Deployment{}).
    WithAction(initialize).
    WithAction(deploy.NewAction()).
    Build(ctx)
```

**What happens:**
1. `ReconcilerFor` is called with `&componentApi.Dashboard{}`
2. Go infers `T = *componentApi.Dashboard`
3. Reconciler is typed to work with Dashboard CRs
4. When reconciling, it automatically:
   - Fetches Dashboard CR from cluster
   - Runs action pipeline
   - Updates Dashboard status
   - Handles errors and requeuing

---

### The Core Reconcile Loop (Simplified)

**File:** `pkg/controller/reconciler/reconciler.go` (Reconcile method)

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)

    // 1. Create new instance of T (Dashboard, Kserve, etc.)
    instance, err := r.instanceFactory()
    if err != nil {
        return ctrl.Result{}, err
    }

    // 2. Fetch from cluster
    if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
        if k8serr.IsNotFound(err) {
            return ctrl.Result{}, nil  // Deleted, nothing to do
        }
        return ctrl.Result{}, err
    }

    // 3. Handle deletion (finalizers)
    if !instance.GetDeletionTimestamp().IsZero() {
        return r.handleDeletion(ctx, instance)
    }

    // 4. Create ReconciliationRequest (the briefcase)
    rr := &ReconciliationRequest{
        Client:     r.Client,
        Controller: r,
        Instance:   instance,
        Release:    r.Release,
        Conditions: r.conditionsManagerFactory(instance),
        Manifests:  []ManifestInfo{},
        Resources:  []unstructured.Unstructured{},
    }

    // 5. Run ALL actions in the pipeline
    for _, action := range r.Actions {
        if err := action(ctx, rr); err != nil {
            // Handle different error types
            if errors.Is(err, &StopError{}) {
                return ctrl.Result{}, nil
            }
            if errors.Is(err, &TransientError{}) {
                return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
            }
            return ctrl.Result{}, err
        }
    }

    // 6. Update status
    if err := r.Client.Status().Update(ctx, instance); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

---

### Type Safety Benefits

```go
// Without generics - runtime type checking
func Reconcile(obj interface{}) error {
    dashboard, ok := obj.(*Dashboard)  // Runtime type assertion
    if !ok {
        return errors.New("wrong type!")  // Whoops, error at runtime!
    }
    dashboard.Status.URL = "..."
}

// With generics - compile-time type checking
func Reconcile[T PlatformObject](obj T) error {
    obj.GetStatus()  // Compiler KNOWS T has GetStatus()
    // If T doesn't implement PlatformObject, COMPILE ERROR!
}
```

**Benefits:**
- **Compile-time safety** - Type errors caught at build time, not runtime
- **IDE autocomplete** - IntelliSense knows what methods are available
- **Refactoring** - Rename methods, compiler finds all usages
- **No type assertions** - No `obj.(*Dashboard)` casting needed

---

### Builder Pattern for Controller Setup

```go
reconciler.ReconcilerFor(mgr, &componentApi.Dashboard{}).
    // Watch primary resource (Dashboard CR)
    For(&componentApi.Dashboard{}).

    // Watch owned resources (resources created by this controller)
    Owns(&corev1.ConfigMap{}).
    Owns(&corev1.Secret{}).
    Owns(&appsv1.Deployment{}, reconciler.WithPredicates(customPredicate)).

    // Watch resources by GVK (for dynamic CRDs)
    OwnsGVK(gvk.OdhApplication, reconciler.Dynamic()).

    // Watch external resources (not owned)
    Watches(&extv1.CustomResourceDefinition{},
        reconciler.WithEventHandler(customHandler),
        reconciler.WithPredicates(customPredicate),
    ).

    // Add actions to the pipeline
    WithAction(initialize).
    WithAction(kustomize.NewAction()).
    WithAction(deploy.NewAction()).

    // Add finalizer actions (run on deletion)
    WithFinalizer(cleanup).

    // Declare custom conditions
    WithConditions("DeploymentsAvailable", "HardwareProfilesReady").

    // Build and register with manager
    Build(ctx)
```

---

## Putting It All Together

```
┌─────────────────────────────────────────────────────────────────┐
│                     OPERATOR STARTUP                             │
└─────────────────────────────────────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
  ┌──────────┐      ┌──────────┐     ┌──────────┐
  │Dashboard │      │  Kserve  │     │Workbenches│
  │  init()  │      │  init()  │     │  init()  │
  └──────────┘      └──────────┘     └──────────┘
        │                 │                 │
        └─────────────────┼─────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │   PATTERN 1: REGISTRY │
              │  All 18 components    │
              │    registered!        │
              └───────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  DSC Controller calls │
              │ registry.ForEach(...)  │
              └───────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
  ch.NewComponentReconciler(mgr) for each component
        │                 │                 │
        ▼                 ▼                 ▼
  ┌──────────┐      ┌──────────┐     ┌──────────┐
  │Dashboard │      │  Kserve  │     │Workbenches│
  │Controller│      │Controller│     │Controller│
  └──────────┘      └──────────┘     └──────────┘
        │                 │                 │
        │    PATTERN 3: GENERIC RECONCILER  │
        │   NewReconciler[Dashboard](mgr)   │
        └───────────────┬───────────────────┘
                        │
      Dashboard CR created by user
                        │
                        ▼
              ┌───────────────────────┐
              │  Reconcile triggered  │
              │  (Controller-Runtime) │
              └───────────────────────┘
                        │
                        ▼
              ┌───────────────────────┐
              │ PATTERN 2: ACTIONS    │
              │                       │
              │ initialize()          │
              │     ↓                 │
              │ setKustomizedParams() │
              │     ↓                 │
              │ configureDeps()       │
              │     ↓                 │
              │ kustomize.NewAction() │
              │     ↓                 │
              │ deploy.NewAction()    │
              │     ↓                 │
              │ deployments.NewAction()
              │     ↓                 │
              │ updateStatus()        │
              │     ↓                 │
              │ gc.NewAction()        │
              └───────────────────────┘
                        │
                        ▼
              Dashboard deployed! ✓
```

---

## Kubernetes Clients Explained

### Three Types of Clients

#### 1. Uncached Client (setupClient)

**File:** `cmd/main.go:229-240`

```go
// Created BEFORE the manager starts
setupClient, err := client.New(setupCfg, client.Options{Scheme: scheme})
```

**Characteristics:**
- **Direct API server access** - Every read/write goes to etcd
- **No cache** - Guaranteed fresh data
- **Used for:** Critical initialization tasks before operator starts
- **Lifecycle:** Created once, used during startup only

**Use cases:**
```go
cluster.Init(ctx, setupClient)                    // Platform detection
cluster.GetDeployedRelease(ctx, setupClient)      // Version checking
initialinstall.CreateDefaultDSCI(ctx, setupClient) // Initial DSCI creation
```

**Why needed?**
- Manager (and its cached client) isn't running yet
- Need to perform pre-flight checks and setup
- Leader election hasn't happened yet

---

#### 2. Cached Client (mgr.GetClient())

**File:** `cmd/main.go:386`

```go
// Used by ALL controllers
Client: mgr.GetClient()
```

**Characteristics:**
- **Cache-backed** - Reads from local in-memory cache, writes go to API server
- **Fast reads** - No network calls for GET/List operations
- **Eventual consistency** - Cache updated via watch streams
- **Reduced API server load** - Critical for operators with many controllers

**How the cache works:**
1. **Watch streams** - Manager opens watch connections to API server for configured types
2. **Local cache** - Stores resources in memory
3. **Reads** - Served from cache (fast!)
4. **Writes** - Go directly to API server (cache auto-updated via watch)

**Cache configuration** (`cmd/main.go:281-333`):
```go
cacheOptions := cache.Options{
    ByObject: map[client.Object]cache.ByObject{
        // Cache only specific namespaces for these types
        &corev1.Secret{}:              { Namespaces: secretCache },
        &corev1.ConfigMap{}:           { Namespaces: oDHCache },
        &appsv1.Deployment{}:          { Namespaces: oDHCache },
        &promv1.PrometheusRule{}:      { Namespaces: oDHCache },

        // Cache only specific instances (field selector)
        &operatorv1.IngressController{}: {
            Field: fields.Set{"metadata.name": "default"}.AsSelector(),
        },
        &configv1.Authentication{}: {
            Field: fields.Set{"metadata.name": "cluster"}.AsSelector(),
        },
    },
}
```

**Excluded from cache** (`cmd/main.go:360-367`):
```go
DisableFor: []client.Object{
    &ofapiv1alpha1.Subscription{},           // OLM subscriptions
    &authorizationv1.SelfSubjectRulesReview{}, // RBAC checks
    &corev1.Pod{},                           // Too many, changes too fast
    &userv1.Group{},                         // User management
    &ofapiv1alpha1.CatalogSource{},          // OLM catalogs
}
```

**Why exclude certain resources?**
- **Pods**: Change frequently (restarts, scaling), would bloat cache
- **RBAC checks**: Need real-time results
- **OLM resources**: External lifecycle, don't want stale data

---

#### 3. API Reader (mgr.GetAPIReader())

**File:** `internal/webhook/*/register.go`

```go
// Used in webhooks and special cases
APIReader: mgr.GetAPIReader()
```

**Characteristics:**
- **Always bypasses cache** - Direct API server access
- **Guaranteed fresh data** - Like uncached client, but from running manager
- **Same lifecycle as manager** - Available after manager starts
- **More expensive** - Network call for every read

**When to use APIReader vs Cached Client:**

| Use APIReader when: | Use Cached Client when: |
|---------------------|-------------------------|
| Need guaranteed fresh data | Normal reconciliation |
| Webhooks (validation/mutation) | Watching resource changes |
| Race condition prevention | List/Get operations |
| Critical decisions | Status updates |

**Examples from codebase:**
```go
// Webhook needs fresh data to validate
internal/webhook/notebook/register.go:14
    APIReader: mgr.GetAPIReader()

// Gateway controller checks OAuth config (critical decision)
internal/controller/services/gateway/gateway_controller.go:42
    cluster.IsIntegratedOAuth(ctx, mgr.GetAPIReader())
```

---

### Client Comparison Table

| Feature | setupClient | mgr.GetClient() | mgr.GetAPIReader() |
|---------|-------------|-----------------|-------------------|
| **Caching** | No | Yes | No |
| **Speed** | Slow (network) | Fast (memory) | Slow (network) |
| **Data freshness** | Guaranteed fresh | Eventual consistency | Guaranteed fresh |
| **API server load** | High | Low | High |
| **Use case** | Startup tasks | Reconciliation | Webhooks, critical reads |
| **Availability** | Before manager | After manager start | After manager start |

---

## Manager Explained

### What Is the Manager?

The **Manager** (`mgr`) is the **central runtime orchestrator** from controller-runtime that manages the entire operator's lifecycle. Think of it as the "brain" of the operator.

**File:** `cmd/main.go:335-373`

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
    Scheme:  scheme,                           // Object types this manager knows
    Metrics: ctrlmetrics.Options{...},        // Prometheus metrics endpoint
    WebhookServer: ctrlwebhook.NewServer(...), // Admission webhooks
    Cache: cacheOptions,                       // What to cache and watch
    LeaderElection: oconfig.LeaderElection,    // Multi-pod coordination
    LeaderElectionID: "07ed84f7.opendatahub.io",
    Client: client.Options{
        Cache: &client.CacheOptions{
            DisableFor: [...]                  // Resources NOT to cache
        }
    }
})
```

---

### Manager Visual Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         MANAGER (mgr)                            │
│                    "The Operator's Brain"                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌──────────┐  ┌────────────┐ │
│  │   Cache    │  │   Client   │  │ Webhooks │  │ Metrics    │ │
│  │  (Watch)   │  │  (CRUD)    │  │ Server   │  │ Server     │ │
│  └────────────┘  └────────────┘  └──────────┘  └────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │              Leader Election (if enabled)                  │ │
│  │     "Only one pod reconciles at a time"                    │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Controllers Registry                      │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │ │
│  │  │   DSCI   │ │   DSC    │ │Dashboard │ │  Kserve  │ ... │ │
│  │  │  Ctrl    │ │  Ctrl    │ │  Ctrl    │ │  Ctrl    │     │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘     │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Runnables (Startup Tasks)                 │ │
│  │  • Create default DSCI                                      │ │
│  │  • Cleanup old resources (v2 → v3 migration)               │ │
│  │  • Health checks (liveness/readiness)                      │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

### What the Manager Provides

#### Shared Components (Available to All Controllers)

| Component | Method | Purpose |
|-----------|--------|---------|
| **Client** | `mgr.GetClient()` | Cached Kubernetes API client |
| **APIReader** | `mgr.GetAPIReader()` | Uncached direct API reader |
| **Scheme** | `mgr.GetScheme()` | Object type registry |
| **Event Recorder** | `mgr.GetEventRecorderFor(name)` | Create Kubernetes events |
| **Cache** | `mgr.GetCache()` | Access watch cache |
| **RESTMapper** | `mgr.GetRESTMapper()` | GVK ↔ resource mapping |

**Example usage:**
```go
// Every controller gets these from the manager
DSCInitializationReconciler{
    Client:   mgr.GetClient(),                              // ← From manager
    Scheme:   mgr.GetScheme(),                              // ← From manager
    Recorder: mgr.GetEventRecorderFor("dscinitialization"), // ← From manager
}
```

---

#### Controller Registration

Controllers are registered with the manager:

```go
// 1. DSCI Controller
(&dscictrl.DSCInitializationReconciler{...}).SetupWithManager(ctx, mgr)

// 2. DSC Controller
dscctrl.NewDataScienceClusterReconciler(ctx, mgr)

// 3. Service Controllers (Auth, Gateway, Monitoring)
CreateServiceReconcilers(ctx, mgr)

// 4. Component Controllers (18 total - Dashboard, Kserve, etc.)
CreateComponentReconcilers(ctx, mgr)
```

**What `SetupWithManager` does:**
```go
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).      // ← Register with manager
        For(&MyCustomResource{}).                  // Watch this resource type
        Owns(&appsv1.Deployment{}).                // Watch owned resources
        Complete(r)                                 // Set this as the reconciler
}
```

---

#### Leader Election

**File:** `cmd/main.go:345-346`

```go
LeaderElection: oconfig.LeaderElection,
LeaderElectionID: "07ed84f7.opendatahub.io",
```

**How it works:**
1. Multiple operator pods run simultaneously (HA deployment)
2. Manager creates a **Lease** resource in Kubernetes
3. **Only ONE pod becomes leader** and reconciles
4. Other pods are on standby (watch cache still runs)
5. If leader crashes, another pod acquires lease (~15s)

**Without leader election:**
- Multiple pods would reconcile the same resource
- Race conditions, duplicate deployments, chaos!

---

#### Runnables - Startup Tasks

**File:** `cmd/main.go:416-454`

Runnables are functions that run **once when manager starts**:

```go
// 1. Create default DSCI if it doesn't exist
var createDefaultDSCIFunc manager.RunnableFunc = func(ctx context.Context) error {
    return initialinstall.CreateDefaultDSCI(ctx, setupClient, platform, ...)
}
mgr.Add(createDefaultDSCIFunc)

// 2. Cleanup old resources from v2 → v3 upgrade
var cleanExistingResourceFunc manager.RunnableFunc = func(ctx context.Context) error {
    return upgrade.CleanupExistingResource(ctx, setupClient, platform, ...)
}
mgr.Add(cleanExistingResourceFunc)

// 3. Health checks (for Kubernetes probes)
mgr.AddHealthzCheck("healthz", healthz.Ping)  // Liveness
mgr.AddReadyzCheck("readyz", healthz.Ping)    // Readiness
```

---

### Manager Lifecycle

```go
setupLog.Info("starting manager")
if err := mgr.Start(ctx); err != nil {  // ← Blocks here until shutdown
    setupLog.Error(err, "problem running manager")
    os.Exit(1)
}
```

**What happens when `mgr.Start(ctx)` is called:**

```
T+0ms:  Manager starts all background goroutines
         ├─ Leader election (if enabled)
         ├─ Cache watch streams to API server
         ├─ Webhook HTTPS server (port 9443)
         ├─ Metrics server (port 8080)
         └─ Health check server (port 8081)

T+100ms: Run all Runnables (startup tasks)
         ├─ Create default DSCI
         ├─ Cleanup old resources
         └─ Mark ready

T+500ms: If leader (or no leader election):
         ├─ Start all controller reconcile loops
         │   ├─ DSCI Controller
         │   ├─ DSC Controller
         │   ├─ 18 Component Controllers
         │   └─ 3 Service Controllers
         └─ Controllers watch for events

T+∞:    Run until shutdown signal (SIGTERM/SIGINT)
         └─ Gracefully stop all controllers & watches
```

---

### Manager vs Controllers

| Manager | Controllers |
|---------|-------------|
| **Singleton** (one per operator) | **Multiple** (one per resource type) |
| Manages infrastructure | Manage resource lifecycles |
| Provides cache, client, webhooks | Use manager's client/cache |
| Runs ALL controllers | Run reconcile loops |
| Leader election coordination | Only reconcile if manager is leader |

**Think of it like:**
- **Manager** = Orchestra conductor
- **Controllers** = Musicians
- **Cache** = Shared sheet music (resource state)
- **Client** = Instrument (API interaction)

---

## Summary

### Pattern Summary Table

| Pattern | Purpose | Key Benefit | Key Files |
|---------|---------|-------------|-----------|
| **Registry** | Component self-registration | No hardcoded lists, easy to add components | `internal/controller/components/registry/` |
| **Actions** | Composable reconciliation pipeline | Reusable logic, readable flow | `pkg/controller/actions/` |
| **Generics** | Type-safe reconciler | One reconciler for all components, compile-time safety | `pkg/controller/reconciler/` |

### Architecture Principles

1. **Separation of Concerns**
   - Registry: Discovery
   - Actions: Logic
   - Generics: Type safety

2. **DRY (Don't Repeat Yourself)**
   - Common logic in reusable actions
   - Generic reconciler for all components
   - Shared utilities in `pkg/`

3. **Extensibility**
   - Add components by implementing interface
   - Compose custom action pipelines
   - Override default behaviors via options

4. **Type Safety**
   - Interfaces enforce contracts
   - Generics catch errors at compile time
   - Strong typing throughout

5. **Testability**
   - Actions can be tested in isolation
   - Mock implementations of interfaces
   - Dependency injection via manager

---

## Quick Reference

### Adding a New Component

```bash
# 1. Generate scaffold
make new-component COMPONENT=mycomponent

# 2. Implement ComponentHandler interface
# Edit: internal/controller/components/mycomponent/mycomponent.go

# 3. Add init() registration
func init() {
    cr.Add(&componentHandler{})
}

# 4. Build action pipeline
# Edit: internal/controller/components/mycomponent/mycomponent_controller.go

# 5. Import in main.go
import _ "github.com/.../internal/controller/components/mycomponent"

# 6. Add to DSC API
# Edit: api/datasciencecluster/v2/datasciencecluster_types.go
```

### Common Action Patterns

```go
// Custom initialization
func initialize(ctx context.Context, rr *ReconciliationRequest) error {
    rr.Manifests = []ManifestInfo{...}
    return nil
}

// Add extra resources
func addResources(ctx context.Context, rr *ReconciliationRequest) error {
    return rr.AddResources(&corev1.Secret{...})
}

// Platform-specific logic
func platformConfig(ctx context.Context, rr *ReconciliationRequest) error {
    if rr.Release.Name == cluster.OpenDataHub {
        // ODH-specific
    } else {
        // RHOAI-specific
    }
    return nil
}

// Update status
func updateStatus(ctx context.Context, rr *ReconciliationRequest) error {
    instance := rr.Instance.(*MyComponent)
    instance.Status.Phase = "Ready"
    return nil
}
```

### Common Watches Patterns

```go
reconciler.ReconcilerFor(mgr, &MyComponent{}).
    // Primary resource
    For(&MyComponent{}).

    // Owned Kubernetes resources
    Owns(&corev1.ConfigMap{}).
    Owns(&appsv1.Deployment{}, reconciler.WithPredicates(customPredicate)).

    // Owned dynamic CRDs (may not exist at startup)
    OwnsGVK(gvk.MyCustomCRD, reconciler.Dynamic()).

    // Watch external resources
    Watches(&extv1.CustomResourceDefinition{},
        reconciler.WithEventHandler(customHandler),
    ).

    Build(ctx)
```

---

## Additional Resources

- [CLAUDE.md](CLAUDE.md) - Project overview and development guide
- [MENTAL_MAP.md](MENTAL_MAP.md) - Visual architecture and navigation
- [RUNTIME_DEEP_DIVE.md](RUNTIME_DEEP_DIVE.md) - Startup sequence, race conditions, debugging
- [Controller-Runtime Book](https://book.kubebuilder.io/) - Upstream controller-runtime documentation
- [Operator SDK](https://sdk.operatorframework.io/) - Operator development framework
