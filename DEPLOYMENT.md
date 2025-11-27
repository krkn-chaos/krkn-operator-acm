# krkn-operator-acm Deployment Guide

<!-- Generated-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929) -->

## Prerequisites

- Kubernetes cluster with Red Hat Advanced Cluster Management (ACM) installed
- kubectl configured to access the cluster
- Cluster with managed clusters registered in ACM
- Each managed cluster must have an `application-manager` secret in its namespace

## Installation Steps

### 1. Install CRDs

```bash
make install
```

This will install the KrknTargetRequest CRD to your cluster.

### 2. Deploy the Operator

#### Option A: Run locally (for development)

```bash
make run
```

#### Option B: Deploy to the cluster

```bash
# Build and push the image (update IMG variable with your registry)
make docker-build docker-push IMG=<your-registry>/krkn-operator-acm:latest

# Deploy to the cluster
make deploy IMG=<your-registry>/krkn-operator-acm:latest
```

### 3. Verify Installation

Check that the operator is running:

```bash
kubectl get pods -n krkn-operator-acm-system
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
kubectl logs -n krkn-operator-acm-system deployment/krkn-operator-acm-controller-manager
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
# Delete the operator deployment
make undeploy

# Remove CRDs
make uninstall
```