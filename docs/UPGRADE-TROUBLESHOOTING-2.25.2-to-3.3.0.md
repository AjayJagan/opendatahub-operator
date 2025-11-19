# Upgrade Troubleshooting Guide: RHOAI 2.25.2 → 3.3.0

This document provides operational fixes for issues encountered during the upgrade from RHOAI 2.25.2 to 3.3.0.

## Related Documentation

📘 **[Component Migration Guide](UPGRADE-MIGRATION-GUIDE-2.25.2-to-3.3.0.md)** - Comprehensive before/during/after upgrade procedures for all components

**Use these guides together:**
- **Migration Guide:** Complete upgrade workflow, component-specific prerequisites, and preparation steps
- **This Troubleshooting Guide:** Operational fixes for errors encountered during actual upgrade execution

## Important Notes

**⚠️ Normal vs Error Recovery Upgrades:**
- In normal upgrades, DSCI and DSC resources should persist and be reconciled by the new operator
- Only delete resources if they are stuck in Error state or reconciliation fails
- The fixes below are for error recovery scenarios, not normal upgrade procedures

## Quick Reference

| Issue | Severity | Component | Symptom |
|-------|----------|-----------|---------|
| [#1](#issue-1-ray-component-blocked-by-codeflare-component) | High | Ray | `RayReady: False` - CodeFlare must be uninstalled |
| [#2](#issue-2-kueue-component-invalid-managementstate) | High | Kueue | `KueueReady: False` - managementState Managed not supported |
| [#3](#issue-3-dashboard-pod-stuck-in-pending-insufficient-cpu) | Medium | Dashboard | `DashboardReady: False` - Pods pending due to CPU or missing secrets |
| [#4](#issue-4-service-mesh-3-operator-stuck-in-pending) | Low | Service Mesh 3 | Service Mesh 3 CSVs in Pending state |
| [#5](#issue-5-modelmeshserving-cr-not-automatically-removed) | Low | ModelMesh | Orphaned ModelMeshServing CRs after upgrade |
| [#6](#issue-6-servicemeshmemberroll-orphaned-when-ossmv2-uninstalled) | Low | Service Mesh 2 | Orphaned ServiceMeshMemberRoll resources |
| [#7](#issue-7-authorino-and-rhcl-subscription-conflicts) | Medium | Authorino/RHCL | Authorino subscription stuck in UpgradePending |

## Pre-Upgrade Checklist

Before starting the upgrade:

1. **Verify Current Installation**
   ```bash
   oc get csv -n redhat-ods-operator
   oc get dsci
   oc get dsc
   ```

2. **Check Cluster Resources**
   ```bash
   oc get nodes
   oc describe nodes | grep -A 5 "Allocated resources"
   ```

3. **Backup Current Configuration**
   ```bash
   oc get dsci default-dsci -o yaml > dsci-backup-$(date +%Y%m%d).yaml
   oc get dsc default-dsc -o yaml > dsc-backup-$(date +%Y%m%d).yaml
   ```

## Upgrade Issues and Fixes

### Issue #1: Ray Component Blocked by CodeFlare Component

**Severity:** High

**Symptom:**
```
RayReady: False
Message: Failed upgrade: CodeFlare component is present in the cluster.
         It must be uninstalled to proceed with Ray component upgrade.
```

**Root Cause:**
In RHOAI 3.3.0, the CodeFlare component has been replaced with the Ray component. The upgrade path requires removing the old CodeFlare component before the Ray component can be installed.

**Diagnosis:**
```bash
# Check DSC status
oc get dsc default-dsc -o yaml | grep -A 5 "RayReady"

# Check if CodeFlare component exists
oc get codeflare -A

# Check operator logs for errors
oc logs -n redhat-ods-operator deployment/rhods-operator | grep -i "codeflare\|ray"
```

**Fix:**
- Type: **Delete Deprecated Component**

```bash
# 1. Check for existing RayCluster resources (should be none during upgrade)
oc get raycluster -A

# 2. Delete the CodeFlare component
oc delete codeflare default-codeflare

# 3. Wait for operator to reconcile (30-60 seconds)
sleep 30

# 4. Verify Ray component is ready
oc get dsc default-dsc -o jsonpath='{.status.conditions[?(@.type=="RayReady")]}'
```

**Expected Result:**
- CodeFlare component removed
- Ray component status changes to Ready
- Ray operator pod running in redhat-ods-applications namespace

---

### Issue #2: Kueue Component Invalid managementState

**Severity:** High

**Symptom:**
```
KueueReady: False
Message: Kueue managementState Managed is not supported, please use Removed or Unmanaged
```

**Root Cause:**
In RHOAI 3.3.0, the Kueue component only supports `managementState: Removed` or `managementState: Unmanaged`. The `Managed` state is not supported in this release.

**Diagnosis:**
```bash
# Check DSC status
oc get dsc default-dsc -o yaml | grep -A 5 "KueueReady"

# Check current Kueue configuration
oc get dsc default-dsc -o jsonpath='{.spec.components.kueue}'

# Check operator logs for errors
oc logs -n redhat-ods-operator deployment/rhods-operator | grep -i "kueue"
```

**Fix:**
- Type: **Update Component Configuration**

```bash
# Option 1: Set Kueue to Removed (recommended if not using Kueue)
oc patch dsc default-dsc --type='json' -p='[{"op": "replace", "path": "/spec/components/kueue/managementState", "value":"Removed"}]'

# Option 2: Set Kueue to Unmanaged (if you want to manage Kueue externally)
oc patch dsc default-dsc --type='json' -p='[{"op": "replace", "path": "/spec/components/kueue/managementState", "value":"Unmanaged"}]'

# Wait for operator to reconcile
sleep 30

# Verify Kueue status
oc get dsc default-dsc -o jsonpath='{.status.conditions[?(@.type=="KueueReady")]}'
```

**Expected Result:**
- Kueue component status updated based on chosen managementState
- If Removed: KueueReady condition shows "Component ManagementState is set to Removed"
- If Unmanaged: Kueue controller continues running but operator doesn't manage it

---

### Issue #3: Dashboard Pod Stuck in Pending (Insufficient CPU)

**Severity:** Medium

**Symptom:**
```
DashboardReady: False
Message: 0/1 deployments ready

Pod Events:
  Warning  FailedScheduling  0/2 nodes are available: 2 Insufficient cpu.
           preemption: 0/2 nodes are available: 2 No preemption victims found.
```

**Root Cause:**
During the upgrade, the new dashboard deployment is created while old pods are still running. If the cluster has limited CPU resources, the new pod cannot be scheduled because old pods are consuming the available CPU.

**Diagnosis:**
```bash
# Check dashboard pods
oc get pods -n redhat-ods-applications | grep dashboard

# Check pod events
oc get events -n redhat-ods-applications --field-selector involvedObject.kind=Pod | grep dashboard

# Check node resources
oc describe nodes | grep -A 5 "Allocated resources"

# Check dashboard deployment rollout status
oc get deployment -n redhat-ods-applications | grep dashboard
```

**Fix:**
- Type: **Delete Old Pods to Free Resources**

```bash
# 1. Identify old dashboard pods (typically with different hash suffixes)
oc get pods -n redhat-ods-applications -l app=rhods-dashboard

# 2. Delete old dashboard pods to free up CPU
oc delete pod -n redhat-ods-applications -l app=rhods-dashboard,pod-template-hash=<OLD_HASH>

# Example (adjust hash based on your environment):
# oc delete pod rhods-dashboard-76c65c9d96-9g625 rhods-dashboard-76c65c9d96-g94nr -n redhat-ods-applications

# 3. Wait for new pod to be scheduled
oc wait --for=condition=Ready pod -l app=rhods-dashboard -n redhat-ods-applications --timeout=300s

# 4. Verify dashboard is ready
oc get dsc default-dsc -o jsonpath='{.status.conditions[?(@.type=="DashboardReady")]}'
```

**Expected Result:**
- Old dashboard pods terminated
- New dashboard pod scheduled and running
- Dashboard deployment shows 1/1 ready

**Alternative Fix (if pods are stuck due to missing OAuth secrets):**

If dashboard pods show errors like:
```
MountVolume.SetUp failed for volume "oauth-config" : secret "dashboard-oauth-config-generated" not found
MountVolume.SetUp failed for volume "oauth-client" : secret "dashboard-oauth-client-generated" not found
```

Force recreation of the deployment:
```bash
# 1. Delete the dashboard deployment
oc delete deployment rhods-dashboard -n redhat-ods-applications

# 2. Wait for operator to recreate (30-60 seconds)
sleep 30

# 3. Wait for pods to be ready
oc wait --for=condition=Ready pod -l app=rhods-dashboard -n redhat-ods-applications --timeout=300s

# 4. Verify dashboard is ready
oc get dsc default-dsc -o jsonpath='{.status.conditions[?(@.type=="DashboardReady")]}'
```

**Alternative Fix (if cluster is consistently resource-constrained):**
Consider adding more nodes or increasing node capacity:
```bash
# Check current node capacity
oc get nodes -o json | jq '.items[] | {name:.metadata.name, cpu:.status.capacity.cpu, allocatable:.status.allocatable.cpu}'
```

---

### Issue #4: Service Mesh 3 Operator Stuck in Pending

**Severity:** Low

**Symptom:**
```
oc get csv -n openshift-operators | grep servicemesh
servicemeshoperator3.v3.1.0   Red Hat OpenShift Service Mesh 3   3.1.0   Pending
servicemeshoperator3.v3.2.1   Red Hat OpenShift Service Mesh 3   3.2.1   Pending

oc get installplan -n openshift-operators | grep servicemeshoperator3
install-g6t7j   servicemeshoperator3.v3.2.1   Manual   false
```

**Root Cause:**
The Service Mesh 3 operator upgrade has manual approval mode enabled. Additionally, intermediate CSV versions (like v3.1.0) can conflict with the target version (v3.2.1) during the upgrade chain, causing constraint satisfaction errors.

**Diagnosis:**
```bash
# Check Service Mesh operator status
oc get csv -n openshift-operators | grep servicemesh

# Check install plans
oc get installplan -n openshift-operators | grep servicemesh

# Check for constraint errors
oc get installplan -n openshift-operators -o yaml | grep -A 10 "ConstraintsNotSatisfiable"
```

**Common Error:**
```
ConstraintsNotSatisfiable: constraints not satisfiable:
@existing/openshift-operators//servicemeshoperator3.v3.1.0 and
@existing/openshift-operators//servicemeshoperator3.v3.2.1 provide ServiceEntry (networking.istio.io/v1alpha3)
```

**Fix:**
- Type: **Approve Install Plan and Resolve Conflicts**

```bash
# 1. Check current install plan status
oc get installplan -n openshift-operators | grep servicemeshoperator3

# 2. Approve the latest Service Mesh 3 install plan
LATEST_INSTALLPLAN=$(oc get installplan -n openshift-operators -o json | jq -r '.items[] | select(.spec.clusterServiceVersionNames[] | contains("servicemeshoperator3")) | .metadata.name' | tail -1)
oc patch installplan $LATEST_INSTALLPLAN -n openshift-operators --type merge -p '{"spec":{"approved":true}}'

# 3. If you see ConstraintsNotSatisfiable errors, delete intermediate CSVs
oc delete csv servicemeshoperator3.v3.1.0 -n openshift-operators 2>/dev/null || true

# 4. Wait for installation to complete
sleep 30

# 5. Verify Service Mesh 3 is installed
oc get csv -n openshift-operators | grep servicemeshoperator3
```

**Expected Result:**
- Service Mesh 3.2.1 CSV shows `Succeeded` status
- Intermediate CSVs are cleaned up
- Service Mesh 2 and Service Mesh 3 operators coexist (both can be installed simultaneously)

**Note:**
- Service Mesh 2 (v2.6.x) and Service Mesh 3 (v3.x) can coexist in the cluster
- Existing Service Mesh 2 control planes continue to work with the v2 operator
- This upgrade does NOT automatically migrate existing SMCP resources to v3
- RHOAI 3.3.0 currently uses Service Mesh 2.6.12 for its control plane

---

### Issue #5: ModelMeshServing CR Not Automatically Removed

**Severity:** Low

**Symptom:**
```
oc get modelmeshserving -A
# Returns existing ModelMeshServing CRs even after upgrade
```

**Root Cause:**
In RHOAI 3.x, the `modelmeshserving` field has been removed from the DSC spec. If a user forgets to set it to `Removed` before the upgrade, the ModelMeshServing custom resources remain in the cluster but are no longer managed by the operator. There is no warning that these resources exist.

**Diagnosis:**
```bash
# Check for ModelMeshServing CRs
oc get modelmeshserving -A

# Check if modelmeshserving field exists in DSC
oc get dsc default-dsc -o yaml | grep -i modelmesh
```

**Fix:**
- Type: **Manual Cleanup of Deprecated Resources**

```bash
# 1. List all ModelMeshServing resources
oc get modelmeshserving -A

# 2. Delete ModelMeshServing CRs
oc delete modelmeshserving --all -A

# 3. Verify removal
oc get modelmeshserving -A
```

**Prevention:**
Before upgrading to 3.3.0, set ModelMeshServing to Removed in the DSC:
```bash
oc patch dsc default-dsc --type='json' -p='[{"op": "add", "path": "/spec/components/modelmeshserving/managementState", "value":"Removed"}]'
```

**Expected Result:**
- All ModelMeshServing CRs are removed
- No orphaned resources remain in the cluster

---

### Issue #6: ServiceMeshMemberRoll Orphaned When OSSMv2 Uninstalled

**Severity:** Low

**Symptom:**
After uninstalling OpenShift Service Mesh v2 operator, ServiceMeshMemberRoll resources remain in the cluster.

**Root Cause:**
ServiceMeshMemberRoll (SMMR) resources are not automatically deleted when the OSSMv2 operator is uninstalled. These orphaned resources can cause confusion and consume cluster resources.

**Diagnosis:**
```bash
# Check for orphaned ServiceMeshMemberRoll resources
oc get servicemeshmemberroll -A

# Check if Service Mesh v2 operator is still installed
oc get csv -n openshift-operators | grep servicemeshoperator.v2
```

**Fix:**
- Type: **Manual Cleanup of Orphaned Resources**

```bash
# 1. List all ServiceMeshMemberRoll resources
oc get servicemeshmemberroll -A

# 2. Delete ServiceMeshMemberRoll resources
oc delete servicemeshmemberroll --all -A

# 3. Verify removal
oc get servicemeshmemberroll -A
```

**Note:**
- Only remove ServiceMeshMemberRoll if you have fully uninstalled OSSMv2
- If you are still using OSSMv2 (like RHOAI 3.3.0 does), do NOT delete these resources

**Expected Result:**
- ServiceMeshMemberRoll resources are removed
- No orphaned Service Mesh resources remain

---

### Issue #7: Authorino and RHCL Subscription Conflicts

**Severity:** Medium

**Symptom:**
```
oc get subscription -n openshift-operators | grep authorino
authorino-operator   Red Hat Connectivity Link creates automatic Authorino subscription   UpgradePending
```

**Root Cause:**
When Red Hat Connectivity Link (RHCL) is installed alongside an existing Authorino installation, RHCL creates an automatic subscription for Authorino. If the original Authorino subscription is removed while RHCL's subscription exists, the RHCL-created Authorino subscription gets stuck in `UpgradePending` state and never recovers.

**Diagnosis:**
```bash
# Check for Authorino subscriptions
oc get subscription -n openshift-operators | grep authorino

# Check for duplicate Authorino operators
oc get csv -n openshift-operators | grep authorino

# Check RHCL installation
oc get subscription -n openshift-operators | grep rhcl
```

**Fix:**
- Type: **Remove Conflicting Subscriptions and Reinstall**

```bash
# 1. Remove stuck Authorino subscription created by RHCL
oc delete subscription authorino-operator -n openshift-operators

# 2. Remove Authorino CSV
oc delete csv -n openshift-operators -l operators.coreos.com/authorino-operator.openshift-operators

# 3. Remove RHCL subscription
oc delete subscription rhcl -n openshift-operators

# 4. Remove RHCL CSV
oc delete csv -n openshift-operators -l operators.coreos.com/rhcl.openshift-operators

# 5. Wait for cleanup (30 seconds)
sleep 30

# 6. Reinstall RHCL (if needed)
# Follow proper RHCL installation documentation
# Ensure no duplicate Authorino subscriptions exist first

# 7. Verify installation
oc get subscription -n openshift-operators | grep -E "(authorino|rhcl)"
oc get csv -n openshift-operators | grep -E "(authorino|rhcl)"
```

**Prevention:**
- Before installing RHCL, check for existing Authorino installations
- Follow proper RHCL installation procedures from official documentation
- Do not manually install Authorino if RHCL will be installed (RHCL manages Authorino)

**Expected Result:**
- Only one Authorino subscription exists (managed by RHCL if using RHCL)
- No subscriptions stuck in `UpgradePending` state
- RHCL and Authorino operators both show `Succeeded` status

**Note:**
This is not a RHOAI bug but a common installation mistake that can be easily recovered. The issue stems from incorrect installation steps for RHCL when Authorino already exists in the cluster.

---

## Post-Upgrade Verification

After applying fixes, verify the upgrade is complete:

1. **Check DSC Status**
   ```bash
   oc get dsc default-dsc -o yaml
   ```

   Expected: `phase: Ready`

2. **Check All Components**
   ```bash
   oc get dsc default-dsc -o jsonpath='{.status.conditions[*].type}' | tr ' ' '\n'
   ```

   Verify all required components show Ready status.

3. **Check Operator Logs**
   ```bash
   oc logs -n redhat-ods-operator deployment/rhods-operator --tail=50
   ```

   Should not show recurring errors.

4. **Check Application Pods**
   ```bash
   oc get pods -n redhat-ods-applications
   ```

   All pods should be Running or Completed.

5. **Verify Version**
   ```bash
   oc get dsci default-dsci -o jsonpath='{.status.release.version}'
   oc get dsc default-dsc -o jsonpath='{.status.release.version}'
   ```

   Should show: `3.3.0`

## Known Issues

### Warning Messages (Can be Ignored)

These warnings appear in logs but don't block the upgrade:

1. **Deprecated Hardware Profiles Warning**
   ```
   DEPRECATED: This version lacks support for the latest scheduling features
   ```
   - Impact: Informational only
   - Action: No action required during upgrade

2. **ServiceMonitor Invalid Configuration**
   ```
   endpoints[0]: it accesses file system via bearer token file which Prometheus specification prohibits
   ```
   - Impact: Metrics collection may be affected
   - Action: Will be fixed in future releases

## Troubleshooting Commands

**Check Overall Upgrade Status:**
```bash
# Check subscription status
oc get subscription -n redhat-ods-operator

# Check CSV status
oc get csv -n redhat-ods-operator

# Check DSCI and DSC
oc get dsci,dsc

# Check all component conditions
oc get dsc default-dsc -o jsonpath='{.status.conditions[?(@.status=="False")]}' | jq
```

**Get Detailed Error Information:**
```bash
# Operator logs
oc logs -n redhat-ods-operator deployment/rhods-operator --tail=100 | grep -i error

# Recent events
oc get events -n redhat-ods-operator --sort-by='.lastTimestamp' | tail -20
oc get events -n redhat-ods-applications --sort-by='.lastTimestamp' | tail -20

# Component statuses
oc get dsc default-dsc -o yaml | grep -A 3 "type: .*Ready"
```

## Rollback Procedure

If the upgrade cannot be completed, rollback to 2.25.2:

```bash
# 1. Delete the 3.3.0 CSV
oc delete csv rhods-operator.v3.3.0 -n redhat-ods-operator

# 2. Restore from backup
oc apply -f dsci-backup-YYYYMMDD.yaml
oc apply -f dsc-backup-YYYYMMDD.yaml

# 3. Wait for reconciliation
oc wait --for=condition=Ready dsci/default-dsci --timeout=300s
```

## Summary

This guide documented **7 upgrade issues** encountered during RHOAI 2.25.2 → 3.3.0 upgrades:

**Critical Issues (High Severity):**
1. Ray component blocked by CodeFlare - requires manual CodeFlare deletion
2. Kueue managementState incompatibility - requires changing to Removed/Unmanaged

**Important Issues (Medium Severity):**
3. Dashboard pods pending due to CPU constraints or missing OAuth secrets
7. Authorino/RHCL subscription conflicts - improper installation order

**Cleanup Issues (Low Severity):**
4. Service Mesh 3 operator requires manual install plan approval
5. ModelMeshServing CRs not automatically removed in 3.x
6. ServiceMeshMemberRoll orphaned when OSSMv2 uninstalled

All issues have been successfully resolved with the operational fixes provided in this guide.

## Support Information

If issues persist after applying these fixes:

1. Collect must-gather logs:
   ```bash
   oc adm must-gather --image=quay.io/modh/must-gather:stable-v2.25
   ```

2. Check operator documentation:
   - [Upgrade Testing Guide](upgrade-testing.md)
   - [General Troubleshooting](troubleshooting.md)

3. Report issues with:
   - Cluster version: `oc version`
   - Operator version: `oc get csv -n redhat-ods-operator`
   - DSCI/DSC status: `oc get dsci,dsc -o yaml`
   - Relevant logs and events

---

**Document Version:** 1.0
**Last Updated:** 2026-02-10
**Upgrade Path:** RHOAI 2.25.2 → 3.3.0
