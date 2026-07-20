/*
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

Assisted-by: Claude Sonnet 4.5 (claude-sonnet-4-5@20250929)
*/

// Package controller implements Kubernetes controllers for the krkn-operator-acm integration.
//
// This package provides controllers that integrate Red Hat Advanced Cluster Management (ACM)
// with the krkn chaos engineering operator. The controllers handle:
//
//   - KrknOperatorTargetProviderConfig: Manages ACM-specific target provider configuration
//   - KrknTargetRequest: Processes chaos target requests with ACM cluster discovery
//   - ConfigMap: Syncs operator configuration from ConfigMaps
//
// The package enables chaos scenarios to be executed across ACM-managed clusters by
// discovering and providing cluster credentials to the core krkn-operator.
package controller

const (
	// OperatorName is the default name for this operator instance
	// Used for:
	// - KrknOperatorTargetProvider registration
	// - UpdateProviderConfig calls
	// - Setting target data in KrknTargetRequest status
	OperatorName = "krkn-operator-acm"

	// ACMIntegrationConfigMapName is the name of the ConfigMap holding ACM integration configuration
	// This ConfigMap contains ACM-specific settings, not general operator configuration
	ACMIntegrationConfigMapName = "krkn-operator-acm-config"

	// ACMDefaultSecret is the default secret name used for ACM managed cluster authentication
	ACMDefaultSecret = "application-manager"

	// CleanupThresholdSeconds is the age threshold in seconds for cleaning up old completed KrknTargetRequest
	// and KrknOperatorTargetProviderConfig resources
	CleanupThresholdSeconds = 3600 // 1 hour

	// StatusCompleted represents a completed resource status
	StatusCompleted = "Completed"

	// ProxyConfigVarPrefix is the prefix for proxy mode configuration variables
	// Each managed cluster gets a variable named ACM_USE_PROXY_<CLUSTER_NAME>
	ProxyConfigVarPrefix = "ACM_USE_PROXY_"

	// ProxyCAConfigMapName is the name of the ConfigMap containing proxy CA certificate
	// This ConfigMap must exist in the operator namespace with annotation
	// service.beta.openshift.io/inject-cabundle: "true"
	ProxyCAConfigMapName = "cluster-proxy-service-ca"

	// ManifestWorkName is the name of the ManifestWork deployed to managed clusters
	// This ManifestWork creates the necessary RBAC for proxy access
	ManifestWorkName = "krkn-service-proxy-rbac"

	// ServiceAccountTokenPath is the path to the operator's service account token
	// This token is used for proxy connections instead of managed cluster tokens
	ServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	// ServiceAccountNamespacePath is the path to the file containing the operator's namespace
	ServiceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	// DefaultServiceAccountName is the default service account name if not detected
	DefaultServiceAccountName = "controller-manager"

	// ProxyServiceLabel is the label selector for finding the cluster proxy service
	ProxyServiceLabel = "component"

	// ProxyServiceLabelValue is the value of the component label for proxy services
	ProxyServiceLabelValue = "cluster-proxy-addon-user"

	// ProxyCASecretName is the name of the Secret containing proxy CA certificate
	ProxyCASecretName = "proxy-server-ca"

	// ProxyCASecretNamespace is the default namespace for proxy CA Secret
	ProxyCASecretNamespace = "multicluster-engine"

	// ProxyCAFallbackSecretName is the fallback Secret name for proxy CA
	ProxyCAFallbackSecretName = "cluster-proxy-ca"

	// ProxyCAFallbackNamespace is the fallback namespace for proxy CA Secret
	ProxyCAFallbackNamespace = "open-cluster-management-agent-addon"

	// ProxyCASecretKey is the key in the Secret containing the CA certificate
	ProxyCASecretKey = "ca.crt"
)
