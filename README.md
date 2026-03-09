# krkn-operator-acm

![test](https://github.com/krkn-chaos/krkn-operator-acm/actions/workflows/test.yml/badge.svg)
![pr-checks](https://github.com/krkn-chaos/krkn-operator-acm/actions/workflows/pr-checks.yml/badge.svg)
![coverage](https://krkn-chaos.github.io/krkn-lib-docs/coverage_badge_krkn-operator-acm.svg)

<!-- Assisted-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929) -->

A Kubernetes operator that integrates Red Hat Advanced Cluster Management (ACM) with krkn-operator for chaos engineering workflows.

## Description

The krkn-operator-acm is designed to automatically discover and configure access to managed clusters in a Red Hat ACM environment. When a `KrknTargetRequest` custom resource is created, the operator:

1. Queries ACM for all managed clusters
2. Retrieves authentication credentials from cluster namespaces
3. Generates kubeconfig files for each managed cluster
4. Stores all cluster access information in a Kubernetes secret
5. Updates the request status with cluster metadata

This automation simplifies the process of targeting multiple clusters for chaos engineering scenarios, eliminating the need for manual credential management and configuration.

## How It Works

### Workflow

```
┌─────────────────────┐
│ Create              │
│ KrknTargetRequest   │
│ (status: pending)   │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│ Operator watches    │
│ for pending         │
│ requests            │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│ Query ACM API for   │
│ managed clusters    │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│ For each cluster:   │
│ - Get secrets       │
│ - Generate config   │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│ Create secret with  │
│ UUID as name        │
└──────────┬──────────┘
           │
           v
┌─────────────────────┐
│ Update request      │
│ status: Completed   │
└─────────────────────┘
```

### Example Usage

1. **Create a KrknTargetRequest**:

```yaml
apiVersion: krkn.krkn-chaos.dev/v1alpha1
kind: KrknTargetRequest
metadata:
  name: chaos-test-1
spec:
  uuid: "chaos-test-uuid-001"
status:
  status: "pending"
```

2. **Operator processes the request** and creates a secret named `chaos-test-uuid-001`

3. **Check the results**:

```bash
# View the updated request
kubectl get krkntargetrequest chaos-test-1 -o yaml

# Example output (multi-operator):
# status:
#   status: Completed
#   created: "2025-12-01T10:00:00Z"
#   completed: "2025-12-01T10:01:30Z"
#   targetData:
#     krkn-operator-acm:
#     - cluster-name: local-cluster
#       cluster-api-url: https://api.acm-hub-krkn.aws.rhperfscale.org:6443
#     - cluster-name: managed-cluster-krkn
#       cluster-api-url: https://api.acm-managed-krkn.aws.rhperfscale.org:6443

# Retrieve the secret
kubectl get secret chaos-test-uuid-001 -o jsonpath='{.data.managed-clusters}' | base64 -d | jq .
```

4. **Use the kubeconfigs** for chaos testing:

```bash
# Extract a specific cluster's kubeconfig from multi-operator format
kubectl get secret chaos-test-uuid-001 -o jsonpath='{.data.managed-clusters}' | \
  base64 -d | jq -r '.["krkn-operator-acm"]["local-cluster"].kubeconfig' | base64 -d > local-cluster.kubeconfig

# Test access
KUBECONFIG=local-cluster.kubeconfig kubectl get nodes
```

## Architecture

### Multi-Operator Support

The krkn-operator-acm supports running multiple operator instances simultaneously, each targeting different cluster management systems (ACM, Hypershift, etc.). This architecture allows for:

- **Horizontal Scalability**: Multiple operators can process the same KrknTargetRequest
- **Provider Diversity**: Different operators can contribute cluster data from different sources
- **Dynamic Completion**: Requests only complete when all active providers have contributed

#### Key Components

1. **KrknOperatorTargetProvider CRD**: Registers each operator instance and tracks its active state
2. **ConfigMap-based Configuration**: Each operator reads its configuration from a ConfigMap
3. **Coordinated Completion**: Requests complete only when data from all active providers is collected

#### How Multi-Operator Works

```
┌─────────────────────┐    ┌─────────────────────┐
│ Operator 1 (ACM)    │    │ Operator 2 (HCP)    │
│ Registers as        │    │ Registers as        │
│ "krkn-operator-acm" │    │ "krkn-operator-hcp" │
└──────────┬──────────┘    └──────────┬──────────┘
           │                           │
           v                           v
      ┌────────────────────────────────────┐
      │ KrknOperatorTargetProvider CRDs    │
      │ - krkn-operator-acm (active: true) │
      │ - krkn-operator-hcp (active: true) │
      └────────────────┬───────────────────┘
                       │
                       v
           ┌────────────────────┐
           │ KrknTargetRequest  │
           │ (status: pending)  │
           └───────┬────────────┘
                   │
      ┌────────────┴────────────┐
      v                         v
┌──────────┐            ┌──────────┐
│ Op1 adds │            │ Op2 adds │
│ ACM data │            │ HCP data │
└────┬─────┘            └─────┬────┘
     │                        │
     └────────────┬───────────┘
                  v
      ┌─────────────────────┐
      │ targetData:         │
      │  krkn-operator-acm: │
      │    - cluster1       │
      │  krkn-operator-hcp: │
      │    - cluster2       │
      └──────────┬──────────┘
                 v
      ┌─────────────────────┐
      │ status: Completed   │
      │ (2/2 providers)     │
      └─────────────────────┘
```

### Custom Resource Definitions (CRDs)

**KrknTargetRequest** (`krkn.krkn-chaos.dev/v1alpha1`):

- **Spec**:
  - `uuid` (string): Unique identifier for the request
- **Status**:
  - `status` (string): Current state - "pending" or "Completed"
  - `created` (timestamp): When the request was created
  - `completed` (timestamp): When all providers finished processing
  - `targetData` (map[string][]ClusterTarget): Cluster data organized by operator name
    - Key: operator-name (e.g., "krkn-operator-acm")
    - Value: Array of cluster targets from that operator

**KrknOperatorTargetProvider** (`krkn.krkn-chaos.dev/v1alpha1`):

- **Spec**:
  - `operatorName` (string): Unique identifier for this operator instance
  - `active` (bool): Whether this provider should be counted for completion logic
  - `timestamp` (timestamp): Last heartbeat/registration time

### Configuration

Each operator instance requires a ConfigMap for configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: krkn-operator-config
  namespace: default
data:
  operator-name: "krkn-operator-acm"
  operator-namespace: "default"
```

**Configuration Fields**:
- `operator-name`: Unique identifier used in targetData keys and provider registration
- `operator-namespace`: Namespace where the operator manages resources

### Secret Structure

The operator creates a secret with multi-operator support:

```json
{
  "krkn-operator-acm": {
    "cluster1": {
      "cluster-name": "local-cluster",
      "cluster-api": "https://api.example.com:6443",
      "kubeconfig": "<base64-encoded-kubeconfig>"
    }
  },
  "krkn-operator-hcp": {
    "cluster2": {
      "cluster-name": "hcp-cluster",
      "cluster-api": "https://api.hcp.example.com:6443",
      "kubeconfig": "<base64-encoded-kubeconfig>"
    }
  }
}
```

### RBAC Permissions

The operator requires:
- Read access to ACM ManagedCluster resources
- Read access to application-manager secrets in cluster namespaces
- Create/update access to secrets in the operator namespace
- Full access to KrknTargetRequest CRs
- Full access to KrknOperatorTargetProvider CRs
- Read access to ConfigMaps in the operator namespace

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/krkn-operator-acm:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/krkn-operator-acm:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/krkn-operator-acm:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/krkn-operator-acm/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
operator-sdk edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

