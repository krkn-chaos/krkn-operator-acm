# Migration Guide

<!-- Assisted-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929) -->

This document provides guidance for migrating between versions of krkn-operator-acm and upgrading from single-operator to multi-operator deployments.

## Table of Contents

- [Overview](#overview)
- [Breaking Changes](#breaking-changes)
- [Migration Scenarios](#migration-scenarios)
- [Data Format Changes](#data-format-changes)
- [Testing Migration](#testing-migration)
- [Rollback Strategy](#rollback-strategy)

## Overview

The krkn-operator-acm has evolved from a single-operator architecture to a multi-operator architecture. This migration guide helps you understand the changes and provides step-by-step instructions for upgrading.

### What Changed?

| Component | Old (Pre-v2.0) | New (v2.0+) |
|-----------|----------------|-------------|
| **TargetData Structure** | `[]ClusterTarget` | `map[string][]ClusterTarget` |
| **Secret Structure** | `map[cluster-name]ClusterData` | `map[operator-name]map[cluster-name]ClusterData` |
| **Provider Registration** | Not required | Required via ConfigMap |
| **Completion Logic** | Always completes | Waits for all active providers |
| **Timestamps** | Not tracked | Created/Completed timestamps |

## Breaking Changes

### 1. KrknTargetRequest Status Structure

**Before (v1.x)**:
```yaml
status:
  status: "Completed"
  targetData:
  - cluster-name: "cluster1"
    cluster-api-url: "https://api.cluster1.com:6443"
  - cluster-name: "cluster2"
    cluster-api-url: "https://api.cluster2.com:6443"
```

**After (v2.0+)**:
```yaml
status:
  status: "Completed"
  created: "2025-12-01T10:00:00Z"
  completed: "2025-12-01T10:01:30Z"
  targetData:
    krkn-operator-acm:
    - cluster-name: "cluster1"
      cluster-api-url: "https://api.cluster1.com:6443"
    - cluster-name: "cluster2"
      cluster-api-url: "https://api.cluster2.com:6443"
```

### 2. Secret Data Structure

**Before (v1.x)**:
```json
{
  "cluster1": {
    "cluster-name": "cluster1",
    "cluster-api": "https://api.cluster1.com:6443",
    "kubeconfig": "..."
  }
}
```

**After (v2.0+)**:
```json
{
  "krkn-operator-acm": {
    "cluster1": {
      "cluster-name": "cluster1",
      "cluster-api": "https://api.cluster1.com:6443",
      "kubeconfig": "..."
    }
  }
}
```

### 3. Configuration Requirement

**v2.0+ requires a ConfigMap**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: krkn-operator-config
  namespace: <operator-namespace>
data:
  operator-name: "krkn-operator-acm"
  operator-namespace: "<operator-namespace>"
```

This ConfigMap is **mandatory** for v2.0+. The operator will fail to start without it.

### 4. New CRD: KrknOperatorTargetProvider

v2.0+ introduces a new CRD for operator registration:

```yaml
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknOperatorTargetProvider
metadata:
  name: krkn-operator-acm
spec:
  operatorName: "krkn-operator-acm"
  active: true
  timestamp: "2025-12-01T10:00:00Z"
```

This CRD is automatically created and managed by the operator.

## Migration Scenarios

### Scenario 1: Upgrading Single Operator (v1.x → v2.0)

This is the most common migration path for existing deployments.

#### Step 1: Backup Current State

```bash
# Backup existing CRs
kubectl get krkntargetrequest -o yaml > krkntargetrequest-backup.yaml

# Backup existing secrets
kubectl get secrets -l app=krkn-operator-acm -o yaml > secrets-backup.yaml
```

#### Step 2: Update CRDs

```bash
# Pull latest version
git pull origin main

# Update CRDs
make install
```

This updates:
- KrknTargetRequest CRD (adds new status fields)
- Installs KrknOperatorTargetProvider CRD

#### Step 3: Create ConfigMap

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: krkn-operator-config
  namespace: krkn-operator-system
data:
  operator-name: "krkn-operator-acm"
  operator-namespace: "krkn-operator-system"
EOF
```

#### Step 4: Deploy New Version

```bash
# Build and push new version
make docker-build docker-push IMG=<registry>/krkn-operator-acm:v2.0.0

# Deploy
make deploy IMG=<registry>/krkn-operator-acm:v2.0.0
```

#### Step 5: Verify Migration

```bash
# Check provider registration
kubectl get krknoperatortargetproviders

# Create test request
cat <<EOF | kubectl apply -f -
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: migration-test
spec:
  uuid: "migration-test-123"
EOF

# Wait and verify
kubectl wait --for=jsonpath='{.status.status}'=Completed \
  krkntargetrequest/migration-test --timeout=120s

kubectl get krkntargetrequest migration-test -o yaml
```

#### Step 6: Update Client Code

If you have automation consuming the KrknTargetRequest data, update it to handle the new structure:

**Old Code**:
```python
# Python example
targets = request['status']['targetData']
for target in targets:
    cluster_name = target['cluster-name']
    cluster_api = target['cluster-api-url']
```

**New Code**:
```python
# Python example
targets = request['status']['targetData']
for operator_name, operator_targets in targets.items():
    for target in operator_targets:
        cluster_name = target['cluster-name']
        cluster_api = target['cluster-api-url']
        # operator_name is now available as context
```

### Scenario 2: Adding Additional Operators

After migrating to v2.0, you can add additional operator instances.

#### Step 1: Deploy Additional Operator

```bash
# Create namespace
kubectl create namespace krkn-operator-hcp-system

# Create ConfigMap
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: krkn-operator-config
  namespace: krkn-operator-hcp-system
data:
  operator-name: "krkn-operator-hcp"
  operator-namespace: "krkn-operator-hcp-system"
EOF

# Deploy operator
make deploy IMG=<registry>/krkn-operator-hcp:v2.0.0 NAMESPACE=krkn-operator-hcp-system
```

#### Step 2: Verify Multi-Operator Setup

```bash
# Should show both providers
kubectl get krknoperatortargetproviders

# NAME                  OPERATORNAME         ACTIVE   TIMESTAMP
# krkn-operator-acm     krkn-operator-acm    true     2025-12-01T10:00:00Z
# krkn-operator-hcp     krkn-operator-hcp    true     2025-12-01T10:00:05Z
```

#### Step 3: Test Multi-Operator Request

```bash
# Create request
kubectl apply -f - <<EOF
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: multi-test
spec:
  uuid: "multi-test-456"
EOF

# Monitor - should wait for both operators
kubectl get krkntargetrequest multi-test -w
```

The request will remain "pending" until both operators contribute data.

### Scenario 3: Migrating from Multi-Operator Back to Single

If you need to revert to a single operator:

#### Step 1: Mark Extra Providers as Inactive

```bash
# Deactivate secondary operator
kubectl patch krknoperatortargetprovider krkn-operator-hcp \
  --type='json' -p='[{"op": "replace", "path": "/spec/active", "value":false}]'
```

#### Step 2: Delete Inactive Operator

```bash
# Undeploy the operator
kubectl delete namespace krkn-operator-hcp-system

# Delete the provider CR (optional, will be recreated if operator redeploys)
kubectl delete krknoperatortargetprovider krkn-operator-hcp
```

#### Step 3: Verify Single Operator Mode

```bash
# Should show only active provider
kubectl get krknoperatortargetproviders

# New requests should complete with single operator
kubectl apply -f test-request.yaml
```

## Data Format Changes

### Accessing Cluster Data

**Old Format (v1.x)**:
```bash
# Extract kubeconfig
kubectl get secret <uuid> -o jsonpath='{.data.managed-clusters}' | \
  base64 -d | jq -r '.["cluster-name"].kubeconfig' | base64 -d
```

**New Format (v2.0+)**:
```bash
# Extract kubeconfig (requires operator-name)
kubectl get secret <uuid> -o jsonpath='{.data.managed-clusters}' | \
  base64 -d | jq -r '.["krkn-operator-acm"]["cluster-name"].kubeconfig' | base64 -d
```

### Iterating Over All Clusters

**Old Format (v1.x)**:
```bash
# List all clusters
kubectl get secret <uuid> -o jsonpath='{.data.managed-clusters}' | \
  base64 -d | jq -r 'keys[]'
```

**New Format (v2.0+)**:
```bash
# List all clusters from all operators
kubectl get secret <uuid> -o jsonpath='{.data.managed-clusters}' | \
  base64 -d | jq -r '.[] | keys[]'

# List clusters by operator
kubectl get secret <uuid> -o jsonpath='{.data.managed-clusters}' | \
  base64 -d | jq -r '.["krkn-operator-acm"] | keys[]'
```

## Testing Migration

### Pre-Migration Validation

Before migrating, validate your current deployment:

```bash
# 1. Check existing requests
kubectl get krkntargetrequest

# 2. Check secrets
kubectl get secrets | grep -E "[0-9a-f-]{36}"

# 3. Export current data for comparison
kubectl get krkntargetrequest -o yaml > pre-migration-requests.yaml
kubectl get secrets -l app=krkn-operator-acm -o yaml > pre-migration-secrets.yaml
```

### Post-Migration Validation

After migration, verify everything works:

```bash
# 1. Verify CRDs
kubectl get crd | grep krkn

# Expected:
# krknoperatortargetproviders.krkn.krkn-chaos.dev
# krkntargetrequests.krkn.krkn-chaos.dev

# 2. Verify provider registration
kubectl get krknoperatortargetproviders

# 3. Run test request
kubectl apply -f config/samples/krkn_v1alpha1_krkntargetrequest.yaml

# 4. Compare structure
kubectl get krkntargetrequest <name> -o yaml > post-migration-request.yaml
diff pre-migration-requests.yaml post-migration-request.yaml
```

### Compatibility Test Matrix

| Test Case | Pre-v2.0 Operator | v2.0 Operator | Expected Result |
|-----------|-------------------|---------------|-----------------|
| Old CR format | ✅ Works | ✅ Works (backward compatible) | Success |
| New CR format | ❌ Fails (unknown fields) | ✅ Works | Success with v2.0 |
| Old secret read | ✅ Works | ⚠️  Empty (new structure) | Migrate secrets |
| New secret read | ❌ Fails | ✅ Works | Success |

## Rollback Strategy

If issues occur during migration, you can rollback:

### Step 1: Revert Operator Deployment

```bash
# Deploy old version
make deploy IMG=<registry>/krkn-operator-acm:v1.9.0
```

### Step 2: Remove New Provider CRs

```bash
# Remove provider registrations
kubectl delete krknoperatortargetproviders --all
```

### Step 3: Restore CRDs (if needed)

```bash
# Checkout old version
git checkout v1.9.0

# Reinstall old CRDs
make install
```

### Step 4: Restore Backups

```bash
# Restore requests
kubectl apply -f krkntargetrequest-backup.yaml

# Restore secrets
kubectl apply -f secrets-backup.yaml
```

### Step 5: Verify Rollback

```bash
# Test with old format
kubectl apply -f old-format-request.yaml
kubectl get krkntargetrequest
```

## Common Migration Issues

### Issue 1: Operator Fails to Start

**Error**: "Failed to load config: configmap krkn-operator-config not found"

**Solution**:
```bash
# Create required ConfigMap
kubectl apply -f config/samples/operator-config.yaml
```

### Issue 2: Request Stuck in Pending

**Error**: KrknTargetRequest never completes

**Possible Causes**:
1. No active providers registered
2. Provider count mismatch

**Solution**:
```bash
# Check providers
kubectl get krknoperatortargetproviders

# If none, restart operator
kubectl rollout restart deployment/krkn-operator-acm-controller-manager -n krkn-operator-system
```

### Issue 3: Secret Structure Mismatch

**Error**: Old scripts can't find cluster data in secrets

**Solution**: Update scripts to handle new nested structure (see [Accessing Cluster Data](#accessing-cluster-data))

### Issue 4: Multiple Operators, Same Name

**Error**: Conflict errors in operator logs

**Solution**:
```bash
# Ensure each operator has unique name
kubectl get configmaps krkn-operator-config -A -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.data.operator-name}{"\n"}{end}'

# Update ConfigMaps to have unique names
```

## Best Practices

1. **Always backup** before migration
2. **Test in staging** environment first
3. **Monitor logs** during and after migration
4. **Update documentation** to reflect new structure
5. **Plan for downtime** during CRD updates
6. **Coordinate with consumers** of the data
7. **Keep rollback plan ready**

## Support

If you encounter issues during migration:

1. Check operator logs: `kubectl logs -n <namespace> deployment/krkn-operator-acm-controller-manager`
2. Review this guide for common issues
3. Check [DEPLOYMENT.md](../DEPLOYMENT.md) for configuration guidance
4. Report issues at https://github.com/krkn-chaos/krkn-operator-acm/issues