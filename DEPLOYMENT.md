# krkn-operator-acm Deployment Guide

<!-- Assisted-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929) -->

This guide covers deployment scenarios for krkn-operator-acm, including single-operator and multi-operator configurations.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Single Operator Deployment](#single-operator-deployment)
- [Multi-Operator Deployment](#multi-operator-deployment)
- [Configuration](#configuration)
- [Verification](#verification)
- [Usage](#usage)
- [Troubleshooting](#troubleshooting)
- [Uninstallation](#uninstallation)

## Prerequisites

- Kubernetes cluster with Red Hat Advanced Cluster Management (ACM) installed
- kubectl configured to access the cluster (cluster-admin recommended)
- Cluster with managed clusters registered in ACM
- Each managed cluster must have an `application-manager` secret in its namespace
- Go 1.24.0+ and Docker 17.03+ (if building from source)

## Single Operator Deployment

### Step 1: Create Namespace

**IMPORTANT:** The namespace must be created manually before deployment to prevent accidental deletion during undeploy.

```bash
kubectl create namespace krkn-operator-system
```

### Step 2: Install CRDs

```bash
make install
```

This installs:
- `KrknTargetRequest` CRD
- `KrknOperatorTargetProvider` CRD

### Step 3: Create Operator ConfigMap

Create a ConfigMap for operator configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: krkn-operator-config
  namespace: krkn-operator-system
data:
  operator-name: "krkn-operator-acm"
  operator-namespace: "krkn-operator-system"
```

Apply the ConfigMap:

```bash
kubectl apply -f config/samples/operator-config.yaml
```

### Step 4: Deploy the Operator

**Note:** `make deploy` will verify that the namespace exists before proceeding. If the namespace doesn't exist, you'll get an error with instructions.

#### Option A: Run locally (for development)

```bash
make run
```

#### Option B: Deploy to the cluster

```bash
# Build and push the image
make docker-build docker-push IMG=<your-registry>/krkn-operator-acm:v1.0.0

# Deploy to the cluster
make deploy IMG=<your-registry>/krkn-operator-acm:v1.0.0
```

Expected output:
```
Checking if namespace krkn-operator-system exists...
✓ Namespace exists, proceeding with deployment...
[deployment output...]
✓ Deployment complete!
```

### Step 5: Verify Installation

```bash
# Check operator pods
kubectl get pods -n krkn-operator-system

# Check operator registration
kubectl get krknoperatortargetproviders -n krkn-operator-system
```

Expected output:
```
NAME                  OPERATORNAME         ACTIVE   TIMESTAMP
krkn-operator-acm     krkn-operator-acm    true     2025-12-01T10:00:00Z
```

### Important Notes on Namespace Management

- **Namespace is NOT managed by the operator deployment** - This prevents accidental deletion when running `make undeploy`
- **The namespace must be created manually** before the first deployment
- **Multiple operators can share the same namespace** (`krkn-operator-system`) safely
- **Undeploy preserves the namespace** and any other operators running in it

## Multi-Operator Deployment

Deploy multiple operator instances to support different cluster management systems (ACM, Hypershift, etc.).

### Architecture

```
┌────────────────────┐  ┌────────────────────┐
│ Operator 1 (ACM)   │  │ Operator 2 (HCP)   │
│ Namespace: op-acm  │  │ Namespace: op-hcp  │
└────────┬───────────┘  └────────┬───────────┘
         │                       │
         └───────────────────────┘
                     │
         ┌───────────▼──────────┐
         │  Shared Resources    │
         │  - KrknTargetRequest │
         │  - Secrets           │
         └──────────────────────┘
```

### Deploy Operator 1 (ACM)

```bash
# Create shared namespace (if not already created)
kubectl create namespace krkn-operator-system

# Create ConfigMap for krkn-operator-acm
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

# Deploy operator
cd /path/to/krkn-operator-acm
make deploy IMG=<your-registry>/krkn-operator-acm:v1.0.0
```

### Deploy Operator 2 (Base krkn-operator)

Both operators can share the same namespace `krkn-operator-system`.

```bash
# Namespace already exists from step 1, no need to create it again

# Deploy krkn-operator (uses its own ConfigMap)
cd /path/to/krkn-operator
make deploy IMG=<your-registry>/krkn-operator:latest
```

**Note:** Each operator deployment will check that the namespace exists before proceeding.

### Verify Multi-Operator Setup

```bash
# Check all pods in the shared namespace
kubectl get pods -n krkn-operator-system

# Expected output:
# NAME                                                   READY   STATUS    RESTARTS   AGE
# krkn-operator-controller-manager-xxx                   2/2     Running   0          5m
# krkn-operator-acm-controller-manager-xxx               1/1     Running   0          2m

# Check all registered providers
kubectl get krknoperatortargetproviders -n krkn-operator-system

# Expected output:
# NAME                  OPERATORNAME         ACTIVE   TIMESTAMP
# krkn-operator-acm     krkn-operator-acm    true     2025-12-01T10:00:00Z
# krkn-operator         krkn-operator        true     2025-12-01T10:00:05Z
```

### Undeploy One Operator (Safe)

You can safely undeploy one operator without affecting the other:

```bash
# Undeploy only krkn-operator-acm
cd /path/to/krkn-operator-acm
make undeploy

# Verify krkn-operator is still running
kubectl get pods -n krkn-operator-system
# NAME                                                   READY   STATUS    RESTARTS   AGE
# krkn-operator-controller-manager-xxx                   2/2     Running   0          10m
```

The namespace and the other operator remain intact!

## Configuration

### ConfigMap Structure

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: krkn-operator-config
  namespace: <operator-namespace>
data:
  # Required: Unique identifier for this operator
  operator-name: "krkn-operator-<suffix>"

  # Required: Namespace where operator manages resources
  operator-namespace: "<operator-namespace>"
```

### Important Notes

- **Unique Names**: Each operator MUST have a unique `operator-name`
- **ConfigMap Name**: Must be exactly `krkn-operator-config`
- **Namespace**: ConfigMap must be in the same namespace as the operator deployment
- **Active State**: Providers can be marked inactive without undeployment

### Managing Provider State

```bash
# Mark a provider as inactive (won't be counted for completion)
kubectl patch krknoperatortargetprovider krkn-operator-acm \
  --type='json' -p='[{"op": "replace", "path": "/spec/active", "value":false}]'

# Reactivate a provider
kubectl patch krknoperatortargetprovider krkn-operator-acm \
  --type='json' -p='[{"op": "replace", "path": "/spec/active", "value":true}]'
```

## Verification

### Health Checks

```bash
# Check all operators
kubectl get pods -A | grep krkn-operator

# Check provider registrations
kubectl get krknoperatortargetproviders -o wide

# View operator logs
kubectl logs -n <namespace> deployment/krkn-operator-acm-controller-manager
```

### Test Request

```bash
# Create test request
cat <<EOF | kubectl apply -f -
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: test-request
  namespace: default
spec:
  uuid: "test-123"
EOF

# Wait for completion
kubectl wait --for=jsonpath='{.status.status}'=Completed \
  krkntargetrequest/test-request --timeout=120s

# View results
kubectl get krkntargetrequest test-request -o yaml
```

## Usage

### Create a KrknTargetRequest

1. Create a KrknTargetRequest CR with status set to "pending":

```yaml
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: my-request
spec:
  uuid: "my-unique-uuid-123"
status:
  status: "pending"
```

2. Apply the CR:

```bash
kubectl apply -f my-request.yaml
```

3. The operator will:
   - Query ACM for all managed clusters
   - Retrieve `application-manager` secrets from each cluster namespace
   - Generate kubeconfigs for each cluster
   - Create a secret named `my-unique-uuid-123` with all cluster data
   - Update the KrknTargetRequest status to "Completed"

### Retrieve Results

1. Check the KrknTargetRequest status:

```bash
kubectl get krkntargetrequest my-request -o yaml
```

The status should show:
- `status: Completed`
- `targetData`: List of clusters with their names and API URLs

2. Retrieve the secret with cluster kubeconfigs:

```bash
kubectl get secret my-unique-uuid-123 -o yaml
```

The secret contains:
- `data.managed-clusters`: JSON map with cluster data (cluster-name, cluster-api, kubeconfig base64 encoded)

### Decode the Secret Data

```bash
# Get the managed-clusters data
kubectl get secret my-unique-uuid-123 -o jsonpath='{.data.managed-clusters}' | base64 -d | jq .
```

This will show a JSON structure like:

```json
{
  "local-cluster": {
    "cluster-name": "local-cluster",
    "cluster-api": "https://api.acm-hub-krkn.aws.rhperfscale.org:6443",
    "kubeconfig": "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOi0gY2x1c3Rlcjogfgo..."
  },
  "managed-cluster-krkn": {
    "cluster-name": "managed-cluster-krkn",
    "cluster-api": "https://api.acm-managed-krkn.aws.rhperfscale.org:6443",
    "kubeconfig": "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOi0gY2x1c3Rlcjogfgo..."
  }
}
```

## RBAC Requirements

The operator requires the following permissions:
- `krkn.krkn-chaos.dev/krkntargetrequests`: get, list, watch, create, update, patch, delete
- `krkn.krkn-chaos.dev/krkntargetrequests/status`: get, update, patch
- `secrets`: get, list, watch, create, update, patch, delete
- `namespaces`: get, list, watch
- `cluster.open-cluster-management.io/managedclusters`: get, list, watch

These permissions are automatically configured in `config/rbac/role.yaml`.

## Troubleshooting

### Operator not processing requests

1. Check operator logs:
```bash
kubectl logs -n krkn-operator-system deployment/krkn-operator-acm-controller-manager
```

2. Verify the KrknTargetRequest status is "pending"

### Missing application-manager secrets

If clusters are being skipped, ensure that each managed cluster namespace has an `application-manager` secret with `ca.crt` and `token` fields:

```bash
kubectl get secret application-manager -n <cluster-name>
```

### Permission errors

Ensure the operator has proper RBAC permissions:

```bash
kubectl get clusterrole krkn-operator-acm-manager-role -o yaml
```

## Uninstallation

```bash
# Delete the operator deployment (preserves namespace and other operators)
make undeploy

# Remove CRDs (only if no other operators need them)
make uninstall

# Optional: Delete the namespace (only if no operators are running in it)
kubectl delete namespace krkn-operator-system
```

**Important:**
- `make undeploy` removes ONLY the `krkn-operator-acm` resources (deployment, service, RBAC)
- The namespace `krkn-operator-system` is preserved
- Other operators (like `krkn-operator`) running in the same namespace are NOT affected
- Only delete the namespace manually if you want to remove ALL operators