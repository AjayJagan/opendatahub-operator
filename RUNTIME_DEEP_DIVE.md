# Runtime Deep Dive: OpenDataHub Operator

This document provides a deep operational understanding of how the OpenDataHub operator behaves at runtime, including initialization sequences, race condition handling, API server interactions, and debugging patterns.

## 🚀 Operator Startup Sequence

Understanding what happens when the operator starts is crucial for debugging startup issues and race conditions.

### Timeline: From Pod Start to Reconciliation

```
T+0ms: Pod starts, main() begins
   │
   ├─> Load configuration (viper, env vars, flags)
   ├─> Setup logging (zap configuration)
   ├─> Setup signal handler (SIGTERM/SIGINT)
   │
T+50ms: Initialize Kubernetes clients
   │
   ├─> Create uncached setupClient for initial setup
   ├─> Connect to K8s API server
   │   └─> If API server unavailable: exponential backoff retry
   │
T+100ms: Initialize cluster configuration
   │
   ├─> cluster.Init(ctx, setupClient)
   │   ├─> Detect platform (OpenDataHub, RHOAI, ManagedRHOAI)
   │   ├─> Determine operator namespace
   │   ├─> Detect OpenShift version
   │   ├─> Check capabilities (routes, templates, etc.)
   │   └─> Set application namespace
   │
T+150ms: Get deployed release version
   │
   ├─> cluster.GetDeployedRelease(ctx, setupClient)
   │   └─> Checks operator ConfigMap for version
   │       └─> Used for upgrade/migration logic
   │
T+200ms: Initialize components and services
   │
   ├─> initServices(ctx, platform)
   │   └─> Calls Init() on each service handler
   │       ├─> Monitoring service
   │       ├─> Auth service
   │       ├─> Gateway service
   │       └─> Setup service
   │
   ├─> initComponents(ctx, platform)
   │   └─> Calls Init() on each component handler
   │       ├─> Dashboard, Kserve, Workbenches, ...
   │       └─> Initializes manifest paths based on platform
   │
T+250ms: Create controller-runtime Manager
   │
   ├─> ctrl.NewManager(...)
   │   ├─> Setup cache (with namespace filters)
   │   ├─> Setup metrics server (default: :8080/metrics)
   │   ├─> Setup health checks (:8081/healthz, /readyz)
   │   ├─> Setup webhook server (port 9443)
   │   └─> Setup leader election (if enabled)
   │
T+300ms: Register webhooks
   │
   ├─> webhook.RegisterAllWebhooks(mgr)
   │   ├─> DSC v1/v2 webhooks
   │   ├─> DSCI v1/v2 webhooks
   │   ├─> HardwareProfile webhooks
   │   ├─> Kueue webhooks
   │   ├─> Serving webhooks
   │   ├─> Notebook webhooks
   │   └─> Dashboard webhooks
   │
   │   ⚠️  IMPORTANT: Webhooks are REGISTERED but NOT YET SERVING
   │
T+350ms: Setup controllers
   │
   ├─> DSCInitialization controller setup
   ├─> DataScienceCluster controller setup
   ├─> Service controllers setup (monitoring, auth, gateway, setup)
   └─> Component controllers setup (dashboard, kserve, workbenches, ...)
   │
   │   ⚠️  IMPORTANT: Controllers are SETUP but NOT YET RUNNING
   │
T+400ms: Add startup runnables
   │
   ├─> mgr.Add(createDefaultDSCIFunc)
   │   └─> Scheduled to run AFTER manager starts
   │
   ├─> mgr.Add(createDefaultDSCFunc) [ManagedRHOAI only]
   │   └─> Scheduled to run AFTER manager starts
   │
   └─> mgr.Add(cleanExistingResourceFunc)
       └─> Cleanup from previous versions
   │
T+450ms: Add health checks
   │
   ├─> /healthz endpoint (liveness probe)
   └─> /readyz endpoint (readiness probe)
   │
T+500ms: Start manager
   │
   ├─> mgr.Start(ctx)
   │   │
   │   ├─> [If leader election enabled]
   │   │   ├─> Attempt to acquire leader lease
   │   │   ├─> Lease name: "07ed84f7.opendatahub.io"
   │   │   ├─> Lease duration: 15s (default)
   │   │   ├─> Renew deadline: 10s (default)
   │   │   └─> Retry period: 2s (default)
   │   │   │
   │   │   └─> If NOT leader: Wait and retry
   │   │       If leader: Continue startup
   │   │
   │   ├─> Start cache (begins watching API server)
   │   │   ├─> Watch DSCInitialization CRs
   │   │   ├─> Watch DataScienceCluster CRs
   │   │   ├─> Watch Component CRs
   │   │   ├─> Watch Service CRs
   │   │   └─> Watch owned resources (Deployments, Services, etc.)
   │   │
   │   ├─> Wait for cache sync
   │   │   └─> Ensures cache has initial state before reconciling
   │   │
   │   ├─> Start webhook server
   │   │   ├─> Listen on port 9443
   │   │   ├─> Load TLS certificates from Secret
   │   │   └─> Begin accepting validation/mutation requests
   │   │
   │   │   ⚠️  CRITICAL TIMING: Webhooks NOW ACTIVE
   │   │
   │   ├─> Start controllers
   │   │   └─> Each controller begins watching and reconciling
   │   │
   │   │   ⚠️  CRITICAL TIMING: Controllers NOW ACTIVE
   │   │
   │   └─> Start runnables (createDefaultDSCI, etc.)
   │       │
   │       └─> These run ONCE after all controllers are started
   │
T+2000ms: System is fully operational
   │
   ├─> /readyz returns 200 OK
   ├─> Pod marked Ready by kubelet
   └─> Reconciliations begin for existing resources
```

### Key Timing Observations

1. **Webhooks before runnables**: Webhooks are active BEFORE the default DSCI/DSC creation runs
2. **Cache sync is blocking**: Controllers won't reconcile until cache is synced
3. **Leader election can delay startup**: Non-leader pods wait indefinitely
4. **Runnables run asynchronously**: createDefaultDSCI runs after controllers start

## 🔄 API Server Interaction Patterns

### How the Operator Talks to Kubernetes

```
┌──────────────────────────────────────────────────────────────┐
│                   Kubernetes API Server                      │
└──────────────────────────────────────────────────────────────┘
        ▲                    ▲                    ▲
        │                    │                    │
    ┌───┴────┐          ┌────┴────┐         ┌────┴────┐
    │ Watch  │          │  Get    │         │  Update │
    │ Events │          │  Read   │         │  Write  │
    └───┬────┘          └────┬────┘         └────┬────┘
        │                    │                    │
┌───────┴────────────────────┴────────────────────┴───────┐
│              Controller-Runtime Manager                 │
│                                                          │
│  ┌────────────────────────────────────────────────┐    │
│  │              Cache (Informer)                   │    │
│  │  - Watches API server for resource changes     │    │
│  │  - Stores local copy of resources              │    │
│  │  - Filtered by namespace & field selectors     │    │
│  └────────────────────────────────────────────────┘    │
│                          │                              │
│                          ▼                              │
│  ┌────────────────────────────────────────────────┐    │
│  │              Work Queue                         │    │
│  │  - Receives events from cache                  │    │
│  │  - Deduplicates rapid changes                  │    │
│  │  - Rate-limits reconciliation requests         │    │
│  └────────────────────────────────────────────────┘    │
│                          │                              │
│                          ▼                              │
│  ┌────────────────────────────────────────────────┐    │
│  │              Controller                         │    │
│  │  - Processes items from queue                  │    │
│  │  - Calls Reconcile() function                  │    │
│  │  - Returns Result{Requeue: bool, ...}          │    │
│  └────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────┘
```

### Cache Behavior

**What's Cached** (from [main.go:281-324](main.go#L281-L324)):
```go
// Namespace-filtered resources:
- Secrets (operator, monitoring, application, openshift-ingress namespaces)
- ConfigMaps (operator, monitoring, application, openshift-operators, openshift-ingress)
- Deployments (operator, monitoring, application)
- PrometheusRules, ServiceMonitors (operator, monitoring, application)
- Routes, NetworkPolicies, Roles, RoleBindings (operator, monitoring, application)

// Field-filtered resources:
- IngressController (only "default")
- Authentication (only "cluster")

// Never cached (always live reads):
- OpenshiftIngress
- Subscription (OLM)
- SelfSubjectRulesReview (auth checks)
- Pod (too many, changes frequently)
- Group (user/group info)
- CatalogSource (OLM)
```

**Why Caching Matters**:
- **Performance**: Avoids repeated API calls
- **Consistency**: Controller sees same state during reconciliation
- **API server load**: Reduces pressure on API server
- **Watch efficiency**: Only watches needed namespaces

**Managed Fields Optimization** ([main.go:325-332](main.go#L325-L332)):
```go
// Strips managedFields to avoid k8s bug:
// https://github.com/kubernetes/kubernetes/issues/124337
DefaultTransform: func(in any) (any, error) {
    if obj, err := meta.Accessor(in); err == nil && obj.GetManagedFields() != nil {
        obj.SetManagedFields(nil)
    }
    return in, nil
}
```

## ⚡ Race Conditions and Prevention

### Race Condition #1: DSCI Created Before Operator Ready

**Problem**: What if DSCI exists before operator starts?

**Solution**: Cache sync + Watch pattern
```
1. Operator starts
2. Cache begins watching DSCInitialization
3. Cache syncs (loads existing resources)
4. Controller starts
5. Existing DSCI triggers reconciliation
```

**Code Location**: [main.go:500-520](main.go#L500-L520)
- Manager waits for cache sync before starting controllers
- Ensures controller sees existing resources

### Race Condition #2: Multiple DSCI Created Simultaneously

**Problem**: Two users create DSCI at the same time

**Solution**: Webhook validation + Singleton check

**Webhook** (runs FIRST):
```go
// internal/webhook/dscinitialization/v1/webhook.go
func (w *DSCInitializationWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
    // List existing DSCInitialization
    instances := &dsciv1.DSCInitializationList{}
    if err := w.Client.List(ctx, instances); err != nil {
        return err
    }

    if len(instances.Items) > 0 {
        return fmt.Errorf("only one instance of DSCInitialization is allowed")
    }

    return nil
}
```

**Controller** (runs SECOND):
```go
// internal/controller/dscinitialization/dscinitialization_controller.go
func (r *DSCInitializationReconciler) Reconcile(ctx context.Context, req ctrl.Request) {
    // Even if webhook failed, controller defensively checks
    instance, err := cluster.GetDSCI(ctx, r.Client)
    if k8serr.IsNotFound(err) {
        return ctrl.Result{}, nil  // No DSCI, nothing to do
    }
    // ... proceed with reconciliation
}
```

### Race Condition #3: DSC Created Before DSCI

**Problem**: User creates DSC before DSCI exists

**Solution**: Controller dependency check

```go
// internal/controller/datasciencecluster/datasciencecluster_controller.go
func (r *DataScienceClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) {
    // First thing: check DSCI exists
    dsci, err := cluster.GetDSCI(ctx, r.Client)
    if err != nil {
        if k8serr.IsNotFound(err) {
            // DSCI doesn't exist - cannot proceed
            r.Recorder.Event(instance, corev1.EventTypeWarning,
                "DSCInitializationMissing",
                "DSCInitialization must exist before DataScienceCluster")
            return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
        }
        return ctrl.Result{}, err
    }

    // DSCI exists, proceed...
}
```

### Race Condition #4: Component CR Created Before DSC

**Problem**: Someone manually creates Component CR before DSC

**Solution**: Owner references + Webhook validation

```go
// internal/webhook/dashboard/webhook.go (example)
func (w *DashboardWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
    dashboard := obj.(*componentv1.Dashboard)

    // Check if owned by DSC
    if len(dashboard.GetOwnerReferences()) == 0 {
        return fmt.Errorf("Dashboard must be created by DataScienceCluster, not directly")
    }

    // Verify owner is DSC
    for _, owner := range dashboard.GetOwnerReferences() {
        if owner.Kind == "DataScienceCluster" {
            return nil  // Valid
        }
    }

    return fmt.Errorf("Dashboard must be owned by DataScienceCluster")
}
```

### Race Condition #5: Concurrent Updates to Same Resource

**Problem**: Two controllers try to update same resource simultaneously

**Solution**: Optimistic concurrency control (resource version)

**Automatic Retry** (controller-runtime handles this):
```go
// When you call client.Update(), the framework:
1. Sends update with current resourceVersion
2. If API server says "conflict" (resourceVersion changed):
   - Framework automatically retries
   - Re-fetches latest version
   - Re-applies your changes
   - Retries up to 5 times by default
```

**Manual Retry Pattern** (for complex updates):
```go
// pkg/dscinitialization/utils.go and many other places
err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
    // 1. Get latest version
    newInstance := &dsciv2.DSCInitialization{}
    if err := r.Client.Get(ctx, client.ObjectKeyFromObject(instance), newInstance); err != nil {
        return err
    }

    // 2. Apply changes to latest version
    newInstance.Spec.SomeField = "new value"

    // 3. Update
    if err := r.Client.Update(ctx, newInstance); err != nil {
        return err  // Retry on conflict
    }

    return nil
})
```

**Server-Side Apply** (no conflicts!):
```go
// pkg/controller/actions/deploy/deploy_ssa.go
// SSA allows multiple managers to own different fields
err := r.Client.Patch(ctx, resource, client.Apply,
    client.FieldOwner("dscinitialization.opendatahub.io"),
    client.ForceOwnership)

// API server merges changes instead of rejecting conflicts
```

### Race Condition #6: Default DSCI Creation

**Problem**: Operator creates default DSCI, but user also creates one

**Solution**: createDefaultDSCIFunc checks first

```go
// pkg/initialinstall/creation.go
func CreateDefaultDSCI(ctx context.Context, cli client.Client, ...) error {
    // List existing DSCI instances
    instances := &dsciv2.DSCInitializationList{}
    if err := cli.List(ctx, instances); err != nil {
        return err
    }

    switch {
    case len(instances.Items) > 1:
        log.Info("only one instance allowed, not creating default")
        return nil
    case len(instances.Items) == 1:
        log.Info("DSCI already exists, not creating default")
        return nil
    case len(instances.Items) == 0:
        log.Info("creating default DSCI")
        err := cluster.CreateWithRetry(ctx, cli, defaultDsci)
        // ...
    }
}
```

**CreateWithRetry Pattern** ([pkg/cluster/resources.go:317-335](pkg/cluster/resources.go#L317-L335)):
```go
func CreateWithRetry(ctx context.Context, cli client.Client, obj client.Object) error {
    backoff := wait.Backoff{
        Duration: 1 * time.Second,
        Factor:   1.5,
        Jitter:   0.1,
        Steps:    10,  // Max 10 attempts
        Cap:      60 * time.Second,
    }

    return wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
        err := cli.Create(ctx, obj)
        if err == nil {
            return true, nil  // Success
        }
        if k8serr.IsAlreadyExists(err) {
            return true, nil  // Already exists, that's fine
        }
        return false, err  // Retry
    })
}
```

## 🔐 Leader Election

### How Leader Election Works

**Configuration** ([main.go:345-357](main.go#L345-L357)):
```go
ctrl.NewManager(..., ctrl.Options{
    LeaderElection:   oconfig.LeaderElection,  // Enabled in production
    LeaderElectionID: "07ed84f7.opendatahub.io",

    // Lease parameters (defaults):
    // LeaseDuration: 15s    - How long lease is valid
    // RenewDeadline: 10s    - Must renew before this
    // RetryPeriod: 2s       - How often to try acquiring/renewing
})
```

**Lease Object** (created in operator namespace):
```yaml
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: 07ed84f7.opendatahub.io
  namespace: <operator-namespace>
spec:
  holderIdentity: <pod-name>
  leaseDurationSeconds: 15
  acquireTime: "2024-01-31T10:00:00Z"
  renewTime: "2024-01-31T10:00:12Z"
```

**Election Flow**:
```
Pod A starts:
  1. Try to acquire lease
  2. If lease doesn't exist: Create it → BECOME LEADER
  3. If lease exists but expired: Update it → BECOME LEADER
  4. If lease exists and valid: Wait → NOT LEADER

Leader Pod (Pod A):
  - Renew lease every 2s
  - Run controllers
  - Run webhooks
  - Process reconciliations

Non-Leader Pod (Pod B):
  - Wait for lease to expire
  - Check every 2s
  - If leader crashes: Acquire lease → BECOME LEADER
  - Webhooks still run (stateless!)
  - Controllers do NOT run
```

**What Runs Where**:
```
┌─────────────────────────────────────────────────────┐
│                    Leader Pod                       │
├─────────────────────────────────────────────────────┤
│ ✅ Controllers (reconciliation)                     │
│ ✅ Webhooks (validation/mutation)                   │
│ ✅ Cache (watches API server)                       │
│ ✅ Metrics endpoint                                 │
│ ✅ Health endpoints                                 │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│                  Non-Leader Pod                     │
├─────────────────────────────────────────────────────┤
│ ❌ Controllers (NOT running)                        │
│ ✅ Webhooks (still running!)                        │
│ ❌ Cache (not needed without controllers)           │
│ ✅ Metrics endpoint                                 │
│ ✅ Health endpoints                                 │
└─────────────────────────────────────────────────────┘
```

**Why Webhooks Run on All Pods**:
- Webhooks are stateless (don't maintain queue/cache)
- Load balancing across pods
- High availability (if leader fails, other pods still validate)

## 🎯 Reconciliation Behavior

### The Reconciliation Loop

```
Event occurs (Create/Update/Delete)
   │
   ▼
Cache receives event
   │
   ▼
Event added to work queue
   │
   ├─> Rate limiting applied
   │   ├─> Deduplicate: Same resource multiple events = 1 queue item
   │   └─> Backoff: Failed reconciliation = exponential backoff
   │
   ▼
Controller processes queue item
   │
   ├─> Calls Reconcile(ctx, Request{Name, Namespace})
   │
   ▼
Reconcile function executes
   │
   ├─> Get resource from API server
   ├─> Run action pipeline
   ├─> Update resource status
   │
   ▼
Return Result
   │
   ├─> Result{} → Success, remove from queue
   ├─> Result{Requeue: true} → Re-add to queue immediately
   ├─> Result{RequeueAfter: 10s} → Re-add to queue after 10s
   └─> error → Re-add to queue with exponential backoff
```

### Reconciliation Result Types

```go
// Success - done, don't requeue
return ctrl.Result{}, nil

// Explicit requeue immediately
return ctrl.Result{Requeue: true}, nil

// Requeue after delay
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

// Error - controller-runtime handles backoff
return ctrl.Result{}, fmt.Errorf("something failed")
```

### Exponential Backoff on Errors

**Default Behavior** (controller-runtime):
```
Attempt 1: Immediate retry
Attempt 2: Wait 1s
Attempt 3: Wait 2s
Attempt 4: Wait 4s
Attempt 5: Wait 8s
Attempt 6: Wait 16s
Attempt 7: Wait 32s
Attempt 8+: Wait 60s (capped)
```

### Reconciliation Triggers

**What Causes a Reconciliation**:

1. **Direct resource change**:
   - DSC created → DSC controller reconciles
   - DSCI updated → DSCI controller reconciles

2. **Owned resource change** ([controller setup](internal/controller/components/dashboard/dashboard_controller.go)):
   ```go
   builder.
       For(&componentv1.Dashboard{}).  // Primary resource
       Owns(&appsv1.Deployment{}).     // Owned resource
       Owns(&corev1.Service{}).        // Owned resource
       ...
   ```
   - Dashboard Deployment updated → Dashboard controller reconciles
   - Owned Service deleted → Dashboard controller reconciles

3. **Watched resource change** ([DSCI controller](internal/controller/dscinitialization/dscinitialization_controller.go)):
   ```go
   builder.
       For(&dsciv2.DSCInitialization{}).
       Watches(&dscv2.DataScienceCluster{},  // Not owned, but watched
           handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
               // Trigger DSCI reconciliation when DSC changes
               return []reconcile.Request{{Name: "default-dsci"}}
           }))
   ```

4. **Periodic reconciliation**:
   - Not configured by default in this operator
   - Can be added via `WithEventFilter` predicates

5. **Manual trigger**:
   - Update resource annotation: `kubectl annotate dsci default-dsci force-reconcile="$(date)"`
   - Controller sees change → reconciles

### Predicate Filters

**What are Predicates**: Functions that filter which events trigger reconciliation

**Example - Generation Predicate** ([pkg/controller/predicates/resources/predicate.go](pkg/controller/predicates/resources/predicate.go)):
```go
// Only reconcile when spec changes, not status changes
GenerationChangedPredicate := predicate.Funcs{
    UpdateFunc: func(e event.UpdateEvent) bool {
        // Only trigger if generation changed (spec updated)
        return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
    },
}
```

**Why This Matters**:
- Status updates don't trigger new reconciliation
- Prevents infinite reconciliation loops
- Reduces unnecessary API server load

## ❌ Error Handling Patterns

### Error Types and Handling

**1. Transient Errors** (expected to resolve):
```go
// Example: Waiting for dependency
if !dependencyReady {
    return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}
```

**2. Permanent Errors** (won't resolve without user action):
```go
// Example: Invalid configuration
if invalidSpec {
    r.Recorder.Event(instance, corev1.EventTypeWarning, "InvalidSpec", err.Error())
    // Update status condition to Degraded
    conditions.MarkFalse(ConditionTypeReady, conditions.WithError(err))
    // Don't requeue - user must fix
    return ctrl.Result{}, nil
}
```

**3. Retryable Errors** (might resolve):
```go
// Example: Network error
if err := deployResource(ctx, resource); err != nil {
    // Log error, update status, return error
    // Controller-runtime will retry with backoff
    return ctrl.Result{}, err
}
```

**4. Stop Errors** (stop action pipeline):
```go
// pkg/controller/actions/errors/errors.go
type StopError struct {
    Err error
}

// In action:
if criticalConditionNotMet {
    return odherrors.NewStopError("waiting for prerequisite")
}

// In reconciler:
if errors.As(err, &odherrors.StopError{}) {
    // Stop executing remaining actions
    // This is NOT an error, just a pause point
    return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}
```

### Status Conditions Pattern

**Standard Conditions** ([internal/controller/status/conditions.go](internal/controller/status/conditions.go)):
- `Ready`: Overall health
- `ProvisioningSucceeded`: All actions completed
- `Available`: Resource is serving traffic
- `Degraded`: Resource exists but has issues
- `Progressing`: Resource is being updated

**Condition Structure**:
```yaml
status:
  conditions:
  - type: Ready
    status: "True"  # or "False" or "Unknown"
    reason: AllComponentsReady
    message: "All components are ready"
    lastTransitionTime: "2024-01-31T10:00:00Z"
    observedGeneration: 5
```

**Setting Conditions** ([pkg/controller/reconciler/reconciler.go:302-314](pkg/controller/reconciler/reconciler.go#L302-L314)):
```go
if provisionErr != nil {
    rr.Conditions.MarkFalse(
        status.ConditionTypeProvisioningSucceeded,
        conditions.WithError(provisionErr),
        conditions.WithObservedGeneration(rr.Instance.GetGeneration()),
    )
} else {
    rr.Conditions.MarkTrue(
        status.ConditionTypeProvisioningSucceeded,
        conditions.WithObservedGeneration(rr.Instance.GetGeneration()),
    )
}
```

### Finalizer Pattern

**Why Finalizers**: Ensure cleanup happens before deletion

**Flow**:
```
User deletes resource (kubectl delete dsc default-dsc)
   │
   ▼
API server sets deletionTimestamp
   │
   ▼
Resource NOT deleted yet (has finalizer)
   │
   ▼
Controller detects deletionTimestamp != nil
   │
   ├─> Run finalizer actions
   │   ├─> Delete child resources
   │   ├─> Clean up external state
   │   └─> Remove owner references
   │
   ├─> Remove finalizer from resource
   │   └─> client.Update(resource)
   │
   ▼
API server deletes resource (no finalizers left)
```

**Code** ([pkg/controller/reconciler/reconciler.go:168-191](pkg/controller/reconciler/reconciler.go#L168-L191)):
```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    res := &MyResource{}
    r.Client.Get(ctx, req.NamespacedName, res)

    if !res.GetDeletionTimestamp().IsZero() {
        // Resource is being deleted
        if !controllerutil.ContainsFinalizer(res, platformFinalizer) {
            return ctrl.Result{}, nil  // Already cleaned up
        }

        // Run cleanup
        if err := r.delete(ctx, res); err != nil {
            return ctrl.Result{}, err  // Retry
        }

        // Remove finalizer
        if err := r.removeFinalizer(ctx, res); err != nil {
            return ctrl.Result{}, err
        }
    } else {
        // Resource is not being deleted, add finalizer
        if err := r.addFinalizer(ctx, res); err != nil {
            return ctrl.Result{}, err
        }

        // Normal reconciliation
        if err := r.apply(ctx, res); err != nil {
            return ctrl.Result{}, err
        }
    }

    return ctrl.Result{}, nil
}
```

## 🐛 Debugging Runtime Issues

### Debugging Scenario #1: "Component Stuck in Progressing"

**Symptoms**:
```bash
$ kubectl get dashboard default-dashboard
NAME               PHASE          AGE
default-dashboard  Progressing    10m
```

**Debug Steps**:

1. **Check status conditions**:
```bash
kubectl get dashboard default-dashboard -o yaml | yq '.status'
```
Look for:
- Which condition is False?
- What's the error message?
- What's the observedGeneration?

2. **Check operator logs**:
```bash
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager \
  | grep "dashboard" | grep "error"
```

3. **Check deployment readiness**:
```bash
kubectl get deployments -n opendatahub -l component=dashboard
```
If deployment not ready → check pod events:
```bash
kubectl get pods -n opendatahub -l component=dashboard
kubectl describe pod <pod-name>
```

4. **Check for missing dependencies**:
```bash
# Does DSCI exist?
kubectl get dsci

# Is monitoring ready?
kubectl get monitoring -A

# Are external operators installed?
kubectl get subscriptions -n openshift-operators
```

5. **Check reconciliation is happening**:
```bash
# Watch operator logs
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager -f

# Trigger manual reconciliation
kubectl annotate dashboard default-dashboard force-reconcile="$(date)"
```

### Debugging Scenario #2: "Webhook Rejects Valid Resource"

**Symptoms**:
```bash
$ kubectl apply -f dsci.yaml
Error from server: admission webhook denied the request: <error message>
```

**Debug Steps**:

1. **Check webhook configuration**:
```bash
kubectl get validatingwebhookconfigurations
kubectl get mutatingwebhookconfigurations
```

2. **Check webhook pod is running**:
```bash
kubectl get pods -n opendatahub -l control-plane=controller-manager
```

3. **Check webhook logs**:
```bash
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager | grep webhook
```

4. **Check TLS certificate**:
```bash
kubectl get secret -n opendatahub webhook-server-cert
```

5. **Bypass webhook for testing** (NOT for production):
```yaml
# Add annotation to resource
metadata:
  annotations:
    opendatahub.io/skip-validation: "true"
```

### Debugging Scenario #3: "Leader Election Issues"

**Symptoms**:
- Multiple operator pods running
- No reconciliation happening
- Logs show "waiting to acquire lease"

**Debug Steps**:

1. **Check lease object**:
```bash
kubectl get lease -n opendatahub 07ed84f7.opendatahub.io -o yaml
```

2. **Check which pod is leader**:
```bash
kubectl get lease -n opendatahub 07ed84f7.opendatahub.io -o jsonpath='{.spec.holderIdentity}'
```

3. **Check lease expiration**:
```bash
kubectl get lease -n opendatahub 07ed84f7.opendatahub.io -o jsonpath='{.spec.renewTime}'
```

4. **Force leader change** (delete lease):
```bash
kubectl delete lease -n opendatahub 07ed84f7.opendatahub.io
# New leader will be elected
```

5. **Check clock skew** (common issue):
```bash
# Check time on all nodes
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].lastHeartbeatTime}{"\n"}{end}'
```

### Debugging Scenario #4: "DSC Not Creating Component CRs"

**Symptoms**:
- DSC exists
- No component CRs created
- DSC status shows no errors

**Debug Steps**:

1. **Check DSC spec**:
```bash
kubectl get dsc default-dsc -o yaml | yq '.spec.components'
```
Ensure managementState is "Managed", not "Removed" or "Unmanaged"

2. **Check DSCI exists**:
```bash
kubectl get dsci
```
If no DSCI → DSC controller waiting

3. **Check DSC controller logs**:
```bash
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager \
  | grep "DataScienceCluster" | grep -i error
```

4. **Check component registry**:
```bash
# List registered components (from operator logs at startup)
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager \
  | grep "creating reconciler" | grep component
```

5. **Force reconciliation**:
```bash
kubectl annotate dsc default-dsc force-reconcile="$(date)"
```

### Debugging Scenario #5: "Race Condition During Startup"

**Symptoms**:
- Resources created in wrong order
- Errors during initial install
- Inconsistent behavior across clusters

**Debug Steps**:

1. **Check operator startup logs**:
```bash
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager \
  | head -100
```

Look for:
- "unable to initialize cluster config" → API server connectivity
- "unable to init components" → Manifest loading issues
- "unable to create controller" → Controller setup failures

2. **Check default DSCI creation**:
```bash
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager \
  | grep "create default DSCI"
```

Should see: "create default DSCI CR." or "DSCI already exists"

3. **Check cache sync**:
```bash
kubectl logs -n opendatahub deployment/opendatahub-operator-controller-manager \
  | grep "cache sync"
```

4. **Check webhook readiness**:
```bash
kubectl get endpoints -n opendatahub opendatahub-operator-controller-manager-metrics-service
```

5. **Test order dependency**:
```bash
# Delete everything
kubectl delete dsc --all
kubectl delete dsci --all
kubectl delete dashboard,kserve,workbenches --all

# Recreate in correct order
kubectl apply -f dsci.yaml
sleep 10  # Wait for DSCI to be ready
kubectl apply -f dsc.yaml
```

## 📊 Observability

### Key Metrics

**Operator Metrics** (exposed on :8080/metrics):
```
# Reconciliation metrics
controller_runtime_reconcile_total{controller="dashboard"} 42
controller_runtime_reconcile_errors_total{controller="dashboard"} 3
controller_runtime_reconcile_time_seconds{controller="dashboard",quantile="0.5"} 0.23

# Work queue metrics
workqueue_depth{name="dashboard"} 0
workqueue_adds_total{name="dashboard"} 42
workqueue_retries_total{name="dashboard"} 3
workqueue_work_duration_seconds{name="dashboard",quantile="0.95"} 0.45

# Leader election
leader_election_master_status{name="07ed84f7.opendatahub.io"} 1
```

**How to Access**:
```bash
# Port-forward to metrics endpoint
kubectl port-forward -n opendatahub deployment/opendatahub-operator-controller-manager 8080:8080

# Query metrics
curl localhost:8080/metrics | grep reconcile

# Or use Prometheus/Grafana (if monitoring enabled)
```

### Logging Levels

**Set Log Level** (via DSCI):
```yaml
apiVersion: dscinitialization.opendatahub.io/v2
kind: DSCInitialization
metadata:
  name: default-dsci
spec:
  devFlags:
    logLevel: 2  # 0=error, 1=info, 2=debug, 3=trace
```

**Dynamic Log Level Change**:
```bash
# Update DSCI
kubectl patch dsci default-dsci --type=merge -p '{"spec":{"devFlags":{"logLevel":3}}}'

# Operator detects change and updates log level immediately
```

### Event Tracking

**View Events**:
```bash
# All events in namespace
kubectl get events -n opendatahub --sort-by='.lastTimestamp'

# Events for specific resource
kubectl describe dsc default-dsc | grep Events -A 20

# Warning events only
kubectl get events -n opendatahub --field-selector type=Warning
```

**Event Types** (emitted by operator):
- `ReconcileSuccess`: Reconciliation completed successfully
- `ReconcileError`: Reconciliation failed
- `ProvisioningError`: Resource provisioning failed
- `DSCInitializationMissing`: DSCI not found
- `InvalidSpec`: Validation failed

## 🧪 Testing Race Conditions

### Test 1: Concurrent DSCI Creation

```bash
# Terminal 1:
kubectl apply -f dsci1.yaml &

# Terminal 2 (immediately):
kubectl apply -f dsci2.yaml &

# Expected: One succeeds, one rejected by webhook
# Verify:
kubectl get dsci
# Should only see one DSCI
```

### Test 2: DSC Before DSCI

```bash
# Create DSC first
kubectl apply -f dsc.yaml

# Verify DSC status shows error
kubectl get dsc default-dsc -o yaml | yq '.status.conditions'
# Should see condition with reason "DSCInitializationMissing"

# Create DSCI
kubectl apply -f dsci.yaml

# Verify DSC reconciles and creates components
kubectl get dashboard,kserve,workbenches
```

### Test 3: Component CR Without DSC

```bash
# Try to create component directly
kubectl apply -f dashboard.yaml

# Expected: Webhook rejects (no owner reference)
# Or if webhook allows: Controller ignores (not owned by DSC)
```

### Test 4: Operator Restart During Reconciliation

```bash
# Start long-running operation
kubectl apply -f dsc-with-many-components.yaml

# Kill operator pod
kubectl delete pod -n opendatahub -l control-plane=controller-manager

# Wait for new pod to start
kubectl wait --for=condition=ready pod -n opendatahub -l control-plane=controller-manager

# Verify reconciliation resumes
kubectl logs -n opendatahub -l control-plane=controller-manager -f
```

---

## 🎓 Key Takeaways

1. **Startup is Sequential**: Cache sync happens before controllers start
2. **Webhooks Run Everywhere**: Leader election doesn't affect webhooks
3. **Retries are Automatic**: Controller-runtime handles most retry logic
4. **Status Tells the Story**: Always check conditions for debugging
5. **Race Conditions are Handled**: Webhooks + defensive programming prevent issues
6. **Leader Election is Critical**: Only one pod reconciles at a time
7. **Finalizers Ensure Cleanup**: Resources are cleaned up before deletion
8. **Caching Reduces Load**: Most reads come from cache, not API server
9. **Exponential Backoff Prevents Storms**: Failed reconciliations back off automatically
10. **Observability is Built-in**: Metrics, logs, and events provide deep visibility

---

**For more details, see**:
- [MENTAL_MAP.md](MENTAL_MAP.md) - Conceptual architecture
- [CLAUDE.md](CLAUDE.md) - Development guide
- [docs/DESIGN.md](docs/DESIGN.md) - Design decisions
