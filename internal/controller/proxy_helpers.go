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
package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

// ProxyConfig holds the configuration for cluster proxy connection
type ProxyConfig struct {
	Enabled    bool
	ProxyURL   string
	ProxyCA    string
	ProxyToken string
}

// isProxyModeEnabled checks if proxy mode is enabled for a given cluster
// via the configstore value ACM_USE_PROXY_<CLUSTER_NAME>
//
// NOTE: Uses global configstore singleton (kvstore.Get()) for consistency with
// rest of codebase (getDefaultSecret, getDefaultProxyMode, ConfigMap sync).
// See commit 91d3b98 for architectural decision rationale.
func isProxyModeEnabled(ctx context.Context, clusterName string) bool {
	logger := log.FromContext(ctx)
	store := kvstore.Get()

	varName := formatProxyVarName(clusterName)
	value, ok := store.GetValue(varName)

	if !ok || value == "" {
		logger.V(1).Info("proxy mode not configured for cluster, using direct connection",
			"cluster", clusterName,
			"config-key", varName)
		return false
	}

	// Parse boolean value (true/false, yes/no, 1/0)
	enabled := value == "true" || value == "yes" || value == "1"
	logger.Info("proxy mode configuration",
		"cluster", clusterName,
		"enabled", enabled,
		"config-key", varName)

	return enabled
}

// formatProxyVarName formats a cluster name to ACM_USE_PROXY_<CLUSTER_NAME> variable
// Converts hyphens to underscores and converts to uppercase
func formatProxyVarName(clusterName string) string {
	formatted := strings.ReplaceAll(clusterName, "-", "_")
	formatted = strings.ToUpper(formatted)
	return ProxyConfigVarPrefix + formatted
}

// getProxyURL constructs the proxy URL for a managed cluster
// Returns: proxy URL string, error
func (r *KrknTargetRequestReconciler) getProxyURL(ctx context.Context, clusterName string) (string, error) {
	logger := log.FromContext(ctx)

	// Get ManagedProxyConfiguration
	proxyConfig := &unstructured.Unstructured{}
	proxyConfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "proxy.open-cluster-management.io",
		Version: "v1alpha1",
		Kind:    "ManagedProxyConfiguration",
	})

	err := r.Get(ctx, types.NamespacedName{
		Name: "cluster-proxy",
	}, proxyConfig)

	if err != nil {
		return "", fmt.Errorf("failed to get ManagedProxyConfiguration: %w", err)
	}

	// Extract proxy server namespace
	proxyNamespace, found, err := unstructured.NestedString(proxyConfig.Object, "spec", "proxyServer", "namespace")
	if err != nil || !found {
		return "", fmt.Errorf("failed to extract proxyServer.namespace from ManagedProxyConfiguration: %w", err)
	}

	logger.Info("found proxy server namespace", "namespace", proxyNamespace)

	// Find the cluster-proxy service
	serviceName, serviceNamespace, servicePort, err := r.findProxyService(ctx, proxyNamespace)
	if err != nil {
		return "", fmt.Errorf("failed to find proxy service: %w", err)
	}

	// Construct proxy URL: https://<service>.<namespace>.svc:<port>/<clusterName>
	proxyURL := fmt.Sprintf("https://%s.%s.svc:%d/%s",
		serviceName, serviceNamespace, servicePort, clusterName)

	logger.Info("constructed proxy URL",
		"cluster", clusterName,
		"proxy-url", proxyURL)

	return proxyURL, nil
}

// findProxyService locates the cluster-proxy service in the given namespace
// Returns: service name, service namespace, service port, error
func (r *KrknTargetRequestReconciler) findProxyService(ctx context.Context, namespace string) (string, string, int32, error) {
	logger := log.FromContext(ctx)

	// List services in the namespace with the proxy label
	serviceList := &corev1.ServiceList{}
	err := r.List(ctx, serviceList,
		client.InNamespace(namespace),
		client.MatchingLabels{ProxyServiceLabel: ProxyServiceLabelValue})

	if err != nil {
		return "", "", 0, fmt.Errorf("failed to list services in namespace %s: %w", namespace, err)
	}

	if len(serviceList.Items) == 0 {
		return "", "", 0, fmt.Errorf("no proxy service found with label %s=%s in namespace %s",
			ProxyServiceLabel, ProxyServiceLabelValue, namespace)
	}

	if len(serviceList.Items) > 1 {
		logger.Info("multiple proxy services found, using first one",
			"namespace", namespace,
			"count", len(serviceList.Items))
	}

	service := serviceList.Items[0]

	if len(service.Spec.Ports) == 0 {
		return "", "", 0, fmt.Errorf("proxy service %s has no ports defined", service.Name)
	}

	port := service.Spec.Ports[0].Port

	logger.Info("found proxy service",
		"service", service.Name,
		"namespace", service.Namespace,
		"port", port)

	return service.Name, service.Namespace, port, nil
}

// getProxyCA retrieves the CA certificate for the cluster proxy service
// The proxy service uses OpenShift service-serving-signer certificates,
// so we need the service CA, not the cluster-proxy CA.
// Reads from ConfigMap: openshift-service-ca.crt (created by OpenShift service CA operator)
// Returns: base64-encoded CA certificate, error
func (r *KrknTargetRequestReconciler) getProxyCA(ctx context.Context) (string, error) {
	logger := log.FromContext(ctx)

	// The proxy service certificate is signed by OpenShift service-serving-signer
	// This CA is injected into ConfigMaps with annotation:
	// service.beta.openshift.io/inject-cabundle: "true"

	// We read from the same ConfigMap that OpenShift creates for service CA
	// in the proxy namespace
	configMapName := "openshift-service-ca.crt"
	configMapNamespace := "multicluster-engine" // Same namespace as proxy service

	// Read ConfigMap
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      configMapName,
		Namespace: configMapNamespace,
	}, configMap)

	if err != nil {
		return "", fmt.Errorf("failed to get service CA ConfigMap %s/%s: %w (verify OpenShift cluster)",
			configMapNamespace, configMapName, err)
	}

	logger.Info("using service CA ConfigMap",
		"configmap", configMapName,
		"namespace", configMapNamespace)

	// Extract CA data from ConfigMap
	// OpenShift service CA operator injects the CA into "service-ca.crt" key
	caData, ok := configMap.Data["service-ca.crt"]
	if !ok {
		return "", fmt.Errorf("service-ca.crt key not found in ConfigMap %s/%s",
			configMapNamespace, configMapName)
	}

	// ConfigMap data is plain text (PEM format), need to base64 encode for kubeconfig
	caBase64 := base64.StdEncoding.EncodeToString([]byte(caData))

	logger.Info("retrieved service CA certificate from ConfigMap",
		"configmap", configMapName,
		"namespace", configMapNamespace,
		"ca-length", len(caData))

	return caBase64, nil
}

// getOperatorToken reads the operator's own service account token
// Returns: token string, error
func (r *KrknTargetRequestReconciler) getOperatorToken(ctx context.Context) (string, error) {
	logger := log.FromContext(ctx)

	tokenBytes, err := os.ReadFile(ServiceAccountTokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read service account token from %s: %w",
			ServiceAccountTokenPath, err)
	}

	token := string(tokenBytes)

	logger.V(1).Info("retrieved operator service account token",
		"token-length", len(token))

	return token, nil
}

// ensureManifestWork ensures the ManifestWork for proxy RBAC exists and is applied
// Creates the ManifestWork if it doesn't exist
// Returns error if ManifestWork exists but is not Applied
func (r *KrknTargetRequestReconciler) ensureManifestWork(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)

	manifestWork := &unstructured.Unstructured{}
	manifestWork.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "work.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManifestWork",
	})

	err := r.Get(ctx, types.NamespacedName{
		Name:      ManifestWorkName,
		Namespace: clusterName,
	}, manifestWork)

	if err != nil {
		if errors.IsNotFound(err) {
			// ManifestWork doesn't exist, create it
			logger.Info("ManifestWork not found, creating",
				"cluster", clusterName,
				"manifestwork", ManifestWorkName)

			err = r.createManifestWork(ctx, clusterName)
			if err != nil {
				return fmt.Errorf("failed to create ManifestWork: %w", err)
			}

			// After creation, ManifestWork needs time to be Applied by OCM agent
			// Log info and return nil - cluster will be retried on next reconcile
			logger.Info("ManifestWork created, waiting for OCM to apply it (cluster will be retried)",
				"cluster", clusterName,
				"manifestwork", ManifestWorkName,
				"hint", "ManifestWork needs time to propagate to managed cluster")
			return nil
		}
		return fmt.Errorf("failed to get ManifestWork: %w", err)
	}

	// ManifestWork exists, check status conditions
	conditions, found, err := unstructured.NestedSlice(manifestWork.Object, "status", "conditions")
	if err != nil || !found {
		logger.Info("ManifestWork has no status conditions yet",
			"cluster", clusterName,
			"manifestwork", ManifestWorkName)
		return fmt.Errorf("ManifestWork exists but has no status conditions, cluster %s not ready for proxy", clusterName)
	}

	// Check for Applied=True and Available=True
	applied := false
	available := false

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)

		if condType == "Applied" && condStatus == "True" {
			applied = true
		}
		if condType == "Available" && condStatus == "True" {
			available = true
		}
	}

	if !applied || !available {
		logger.Error(fmt.Errorf("ManifestWork not ready"),
			"ManifestWork not ready for proxy, skipping cluster",
			"cluster", clusterName,
			"applied", applied,
			"available", available)
		return fmt.Errorf("ManifestWork not ready (Applied=%v, Available=%v), cluster %s not reachable via proxy",
			applied, available, clusterName)
	}

	logger.Info("ManifestWork validated successfully",
		"cluster", clusterName,
		"manifestwork", ManifestWorkName)

	return nil
}

// getServiceAccountName retrieves the service account name the operator is running as
// Returns the service account name or falls back to default
func (r *KrknTargetRequestReconciler) getServiceAccountName(ctx context.Context) string {
	logger := log.FromContext(ctx)

	// Try to read from environment first (some deployment tools set this)
	if saName := os.Getenv("SERVICE_ACCOUNT_NAME"); saName != "" {
		logger.V(1).Info("using service account from environment", "service-account", saName)
		return saName
	}

	// Fallback to default (standard for krkn-operator-acm)
	logger.V(1).Info("using default service account name", "service-account", DefaultServiceAccountName)
	return DefaultServiceAccountName
}

// createManifestWork creates a ManifestWork for proxy RBAC in the cluster namespace
func (r *KrknTargetRequestReconciler) createManifestWork(ctx context.Context, clusterName string) error {
	logger := log.FromContext(ctx)

	// Get operator service account name
	serviceAccountName := r.getServiceAccountName(ctx)

	// Construct the proxy user name: cluster:hub:system:serviceaccount:<namespace>:<sa-name>
	proxyUserName := fmt.Sprintf("cluster:hub:system:serviceaccount:%s:%s",
		r.OperatorNamespace, serviceAccountName)

	// Build ManifestWork template with dynamic service account
	// Uses cluster-admin ClusterRole for full chaos engineering capabilities.
	//
	// SECURITY NOTE: cluster-admin is required because krkn chaos testing needs to:
	// - Kill/delete critical system pods (etcd, system operators, control plane)
	// - Modify resources in any namespace (including kube-system, openshift-*)
	// - Simulate infrastructure failures (drain nodes, delete PVs, etc.)
	// - Test resilience of cluster-critical components
	//
	// Krkn's scope is intentionally destructive testing across all cluster resources.
	// Restricting permissions would defeat the purpose of chaos engineering.
	// Only enable proxy mode on clusters where such testing is authorized.
	manifestWorkTemplate := fmt.Sprintf(`apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: %s
  labels:
    app.kubernetes.io/name: krkn
    app.kubernetes.io/component: service-proxy-rbac
spec:
  workload:
    manifests:
      - apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: krkn-service-proxy-access
          labels:
            app.kubernetes.io/name: krkn
            app.kubernetes.io/component: service-proxy-rbac
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: cluster-admin
        subjects:
          - apiGroup: rbac.authorization.k8s.io
            kind: User
            name: %s`, ManifestWorkName, proxyUserName)

	// Parse the ManifestWork template
	manifestWork := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(manifestWorkTemplate), &manifestWork.Object)
	if err != nil {
		return fmt.Errorf("failed to parse ManifestWork template: %w", err)
	}

	// Set namespace to cluster name
	manifestWork.SetNamespace(clusterName)
	manifestWork.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "work.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManifestWork",
	})

	// Create the ManifestWork
	err = r.Create(ctx, manifestWork)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("ManifestWork already exists (race condition)",
				"cluster", clusterName,
				"manifestwork", ManifestWorkName)
			return nil
		}
		return fmt.Errorf("failed to create ManifestWork: %w", err)
	}

	logger.Info("created ManifestWork for proxy RBAC",
		"cluster", clusterName,
		"manifestwork", ManifestWorkName,
		"proxy-user", proxyUserName)

	return nil
}

// getProxyConfig determines the proxy configuration for a cluster
// Returns: ProxyConfig struct with all proxy settings, error
func (r *KrknTargetRequestReconciler) getProxyConfig(ctx context.Context, clusterName string) (*ProxyConfig, error) {
	logger := log.FromContext(ctx)

	config := &ProxyConfig{
		Enabled: false,
	}

	// Check if proxy mode is enabled
	if !isProxyModeEnabled(ctx, clusterName) {
		return config, nil
	}

	// Proxy mode is enabled - from this point forward, we fail fast on any error.
	// We NEVER return Enabled=false after this point to prevent silent cluster drops.
	// Any missing prerequisites (ManifestWork, proxy service, CA) return error.
	config.Enabled = true

	// Ensure ManifestWork exists and is Applied
	// NOTE: ensureManifestWork returns nil IMMEDIATELY after creating a new ManifestWork
	// (before checking status), so we need to verify status separately
	err := r.ensureManifestWork(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("ManifestWork validation failed for cluster %s: %w", clusterName, err)
	}

	// After ensureManifestWork succeeds, verify the ManifestWork has status conditions
	// If just created (no status yet), we return error to prevent cluster from being
	// silently dropped when falling back to direct connection and secret is missing.
	// This ensures proper retry behavior.
	manifestWork := &unstructured.Unstructured{}
	manifestWork.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "work.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManifestWork",
	})
	err = r.Get(ctx, types.NamespacedName{
		Name:      ManifestWorkName,
		Namespace: clusterName,
	}, manifestWork)
	if err != nil {
		return nil, fmt.Errorf("failed to get ManifestWork after validation: %w", err)
	}

	// Check if it has status conditions (meaning OCM agent has processed it)
	conditions, found, err := unstructured.NestedSlice(manifestWork.Object, "status", "conditions")
	if err != nil {
		return nil, fmt.Errorf("failed to parse ManifestWork status conditions: %w (ManifestWork may be malformed)", err)
	}
	if !found || len(conditions) == 0 {
		// Just created, no status yet - return error to skip cluster and retry
		// This ensures cluster is not omitted from KrknTargetRequest if secret is missing
		return nil, fmt.Errorf("proxy mode enabled but ManifestWork not yet Applied by OCM (just created). "+
			"Cluster %s will be retried in next reconcile after OCM processes ManifestWork",
			clusterName)
	}

	// Get proxy URL
	proxyURL, err := r.getProxyURL(ctx, clusterName)
	if err != nil {
		// Proxy mode explicitly enabled but resources missing - this is an ERROR
		return nil, fmt.Errorf("proxy mode enabled but ManagedProxyConfiguration or Service not found: %w. "+
			"Either install OCM cluster-proxy-addon or set ACM_USE_PROXY_%s=false",
			err, strings.ToUpper(strings.ReplaceAll(clusterName, "-", "_")))
	}
	config.ProxyURL = proxyURL

	// Get proxy CA
	proxyCA, err := r.getProxyCA(ctx)
	if err != nil {
		// Proxy mode explicitly enabled but CA Secret missing - this is an ERROR
		return nil, fmt.Errorf("proxy mode enabled but CA Secret not found: %w. "+
			"Either install OCM cluster-proxy-addon (creates Secret automatically) or set ACM_USE_PROXY_%s=false",
			err, strings.ToUpper(strings.ReplaceAll(clusterName, "-", "_")))
	}
	config.ProxyCA = proxyCA

	// Get operator token
	token, err := r.getOperatorToken(ctx)
	if err != nil {
		// Proxy mode explicitly enabled but token not available - this is an ERROR
		return nil, fmt.Errorf("proxy mode enabled but service account token not available: %w. "+
			"Proxy mode requires operator running in Kubernetes pod. "+
			"Either deploy operator in-cluster or set ACM_USE_PROXY_%s=false for local development",
			err, strings.ToUpper(strings.ReplaceAll(clusterName, "-", "_")))
	}
	config.ProxyToken = token

	logger.Info("proxy configuration built successfully",
		"cluster", clusterName,
		"proxy-url", proxyURL)

	return config, nil
}
