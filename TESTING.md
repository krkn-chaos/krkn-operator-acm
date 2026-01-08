# Testing Guide for krkn-operator-acm

<!-- Generated-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929) -->

## Prerequisites

- Kubeconfig: `/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig`
- ACM cluster with managed clusters configured
- Each managed cluster should have an `application-manager` secret

## Quick Start Testing

### 1. Install CRDs (Already Done)

```bash
export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig
make install
```

✅ Status: CRD `krkntargetrequests.krkn.krkn-chaos.dev` is installed

### 2. Run the Operator Locally

```bash
export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig
make run
```

This will:
- Start the operator on your local machine
- Connect to the ACM cluster
- Watch for KrknTargetRequest resources
- Show logs in your terminal

**To stop**: Press `Ctrl+C`

### 3. Test the Operator

#### 3a. Check Managed Clusters

First, verify that ACM has managed clusters:

```bash
export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig
kubectl get managedclusters
```

Expected output:
```
NAME                   HUB ACCEPTED   MANAGED CLUSTER URLS                                        JOINED   AVAILABLE   AGE
local-cluster          true           https://api.acm-hub-krkn.aws.rhperfscale.org:6443           True     True        Xd
managed-cluster-krkn   true           https://api.acm-managed-krkn.aws.rhperfscale.org:6443       True     True        Xd
```

#### 3b. Verify Secrets Exist

Check that each cluster namespace has the `application-manager` secret:

```bash
# For local-cluster
kubectl get secret application-manager -n local-cluster

# For other clusters
kubectl get secret application-manager -n managed-cluster-krkn
```

#### 3c. Create a Test Request

Create a file `test-request.yaml`:

```yaml
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: test-request-1
spec:
  uuid: "test-uuid-001"
status:
  status: "pending"
```

Apply it:

```bash
kubectl apply -f test-request.yaml
```

#### 3d. Monitor the Operator

Watch the operator logs (in the terminal where `make run` is running). You should see:

```
INFO    Processing KrknTargetRequest    {"UUID": "test-uuid-001"}
INFO    Found managed clusters          {"count": 2}
INFO    Processing cluster              {"name": "local-cluster"}
INFO    Processing cluster              {"name": "managed-cluster-krkn"}
INFO    Created secret with managed clusters data       {"secretName": "test-uuid-001"}
INFO    Successfully completed KrknTargetRequest        {"UUID": "test-uuid-001"}
```

#### 3e. Verify the Results

**Check the updated request:**

```bash
kubectl get krkntargetrequest test-request-1 -o yaml
```

You should see:
```yaml
status:
  status: Completed
  targetData:
  - cluster-name: local-cluster
    cluster-api-url: https://api.acm-hub-krkn.aws.rhperfscale.org:6443
  - cluster-name: managed-cluster-krkn
    cluster-api-url: https://api.acm-managed-krkn.aws.rhperfscale.org:6443
```

**Check the created secret:**

```bash
kubectl get secret test-uuid-001 -o yaml
```

**Decode and view the cluster data:**

```bash
kubectl get secret test-uuid-001 -o jsonpath='{.data.managed-clusters}' | base64 -d | jq .
```

Expected output:
```json
{
  "local-cluster": {
    "cluster-name": "local-cluster",
    "cluster-api": "https://api.acm-hub-krkn.aws.rhperfscale.org:6443",
    "kubeconfig": "YXBpVmVyc2lvbjogdjEK..."
  },
  "managed-cluster-krkn": {
    "cluster-name": "managed-cluster-krkn",
    "cluster-api": "https://api.acm-managed-krkn.aws.rhperfscale.org:6443",
    "kubeconfig": "YXBpVmVyc2lvbjogdjEK..."
  }
}
```

**Extract and test a kubeconfig:**

```bash
# Extract local-cluster kubeconfig
kubectl get secret test-uuid-001 -o jsonpath='{.data.managed-clusters}' | \
  base64 -d | jq -r '.["local-cluster"].kubeconfig' | base64 -d > /tmp/local-cluster-kubeconfig

# Test it
KUBECONFIG=/tmp/local-cluster-kubeconfig kubectl get nodes
```

## Troubleshooting

### Operator Not Processing Requests

1. **Check CRD is installed:**
   ```bash
   kubectl get crd krkntargetrequests.krkn.krkn-chaos.dev
   ```

2. **Check request status is "pending":**
   ```bash
   kubectl get krkntargetrequest test-request-1 -o jsonpath='{.status.status}'
   ```

3. **View operator logs** (in the `make run` terminal)

### Missing Managed Clusters

1. **Verify ACM is configured:**
   ```bash
   kubectl get managedclusters
   ```

2. **Check if you have RBAC permissions:**
   ```bash
   kubectl auth can-i list managedclusters.cluster.open-cluster-management.io
   ```

### Missing Secrets

If the operator skips clusters:

1. **Check namespace exists:**
   ```bash
   kubectl get namespace <cluster-name>
   ```

2. **Check secret exists:**
   ```bash
   kubectl get secret application-manager -n <cluster-name>
   ```

3. **Check secret has required fields:**
   ```bash
   kubectl get secret application-manager -n <cluster-name> -o jsonpath='{.data}' | jq 'keys'
   ```
   Should show: `["ca.crt", "token"]`

## Alternative: Deploy to Cluster

If you want to deploy the operator as a pod in the cluster (instead of running locally):

```bash
# Build and push image
make docker-build docker-push IMG=<your-registry>/krkn-operator-acm:test

# Deploy to cluster
export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig
make deploy IMG=<your-registry>/krkn-operator-acm:test

# Check operator pod
kubectl get pods -n krkn-operator-system

# View logs
kubectl logs -n krkn-operator-system deployment/krkn-operator-acm-controller-manager -f
```

## Cleanup

### Remove Test Resources

```bash
# Delete test request
kubectl delete krkntargetrequest test-request-1

# Delete created secret
kubectl delete secret test-uuid-001
```

### Stop Local Operator

Press `Ctrl+C` in the terminal running `make run`

### Uninstall CRDs

```bash
export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig
make uninstall
```

### Undeploy from Cluster (if deployed)

```bash
export KUBECONFIG=/Users/tsebasti/Scrap/acm-hub-krkn/auth/kubeconfig
make undeploy
```

## Next Steps

Once testing is successful:

1. Create production KrknTargetRequest resources
2. Integrate with krkn-operator for chaos engineering
3. Set up monitoring and alerting
4. Deploy operator permanently to the cluster