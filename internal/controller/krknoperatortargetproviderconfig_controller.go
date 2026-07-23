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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
	"github.com/krkn-chaos/krkn-operator/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/typing"
)

// KrknOperatorTargetProviderConfigReconciler reconciles a KrknOperatorTargetProviderConfig object
type KrknOperatorTargetProviderConfigReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	OperatorName      string
	OperatorNamespace string
}

// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krknoperatortargetproviderconfigs,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krknoperatortargetproviderconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=list
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KrknOperatorTargetProviderConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the KrknOperatorTargetProviderConfig instance
	config := &krknv1alpha1.KrknOperatorTargetProviderConfig{}
	err := r.Get(ctx, req.NamespacedName, config)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("krknOperatorTargetProviderConfig resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get KrknOperatorTargetProviderConfig")
		return ctrl.Result{}, err
	}

	// Initialize status to "pending" if it's empty (new resource)
	if config.Status.Status == "" {
		logger.Info("initializing status to pending", "UUID", config.Spec.UUID)

		// Add UUID label if not present
		if config.Labels == nil {
			config.Labels = make(map[string]string)
		}

		needsLabelUpdate := false
		if _, exists := config.Labels["krkn.krkn-chaos.dev/uuid"]; !exists {
			config.Labels["krkn.krkn-chaos.dev/uuid"] = config.Spec.UUID
			needsLabelUpdate = true
		}

		// Update labels if needed
		if needsLabelUpdate {
			err = r.Update(ctx, config)
			if err != nil {
				if errors.IsConflict(err) {
					logger.Info("conflict updating labels, will retry")
					return ctrl.Result{Requeue: true}, nil
				}
				logger.Error(err, "failed to add UUID label")
				return ctrl.Result{}, err
			}
		}

		// Update status
		now := metav1.Now()
		config.Status.Status = "pending"
		config.Status.Created = &now
		err = r.Status().Update(ctx, config)
		if err != nil {
			if errors.IsConflict(err) {
				logger.Info("conflict initializing status, will retry")
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Error(err, "failed to initialize status")
			return ctrl.Result{}, err
		}
		// Return empty result to allow natural reconciliation
		return ctrl.Result{}, nil
	}

	// Check if the config is already completed
	if config.Status.Status == StatusCompleted {
		logger.Info("krknOperatorTargetProviderConfig already completed", "UUID", config.Spec.UUID)
		return ctrl.Result{}, nil
	}

	// Check if status is pending
	if config.Status.Status != "pending" {
		logger.Info("krknOperatorTargetProviderConfig status is not pending, skipping", "status", config.Status.Status)
		return ctrl.Result{}, nil
	}

	// Check if this provider itself is active before processing
	providerList, shouldSkip, result, err := checkProviderActive(ctx, r.Client, logger, r.OperatorName, r.OperatorNamespace)
	if err != nil {
		return result, err
	}
	if shouldSkip {
		return result, nil
	}

	logger.Info("processing KrknOperatorTargetProviderConfig", "UUID", config.Spec.UUID)

	// Build the configuration schema based on ACM managed clusters and their secrets
	jsonSchema, err := r.buildConfigSchema(ctx)
	if err != nil {
		logger.Error(err, "failed to build configuration schema")
		return ctrl.Result{}, err
	}

	logger.Info("built configuration schema", "schema-length", len(jsonSchema))

	// Re-fetch the config to get the latest version before updating
	// This helps avoid conflicts if the resource was modified by webhooks or other controllers
	freshConfig := &krknv1alpha1.KrknOperatorTargetProviderConfig{}
	err = r.Get(ctx, req.NamespacedName, freshConfig)
	if err != nil {
		logger.Error(err, "failed to re-fetch KrknOperatorTargetProviderConfig")
		return ctrl.Result{}, err
	}

	// Update the provider config using the krkn-operator package
	err = provider.UpdateProviderConfig(
		ctx,
		r.Client,
		freshConfig,
		r.OperatorName,
		ACMIntegrationConfigMapName,
		r.OperatorNamespace,
		jsonSchema,
	)
	if err != nil {
		// If there's a conflict, requeue to retry with the latest version
		if errors.IsConflict(err) {
			logger.Info("conflict updating provider config, will retry", "UUID", freshConfig.Spec.UUID)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "failed to update provider config")
		return ctrl.Result{}, err
	}

	// Re-fetch to get the updated config with all provider contributions
	updatedConfig := &krknv1alpha1.KrknOperatorTargetProviderConfig{}
	err = r.Get(ctx, req.NamespacedName, updatedConfig)
	if err != nil {
		logger.Error(err, "failed to re-fetch updated config")
		return ctrl.Result{}, err
	}

	// Count only active providers (providerList was already fetched earlier)
	activeProviderCount := countActiveProviders(providerList)
	contributorCount := len(updatedConfig.Status.ConfigData)

	logger.Info("provider status",
		"total-providers", len(providerList.Items),
		"active-providers", activeProviderCount,
		"contributors", contributorCount,
		"contributor-names", getConfigDataKeys(updatedConfig.Status.ConfigData),
		"provider-namespaces", getProviderNamespaces(providerList.Items))

	// Check if all active providers have contributed
	if shouldMarkAsCompleted(activeProviderCount, contributorCount) {
		completedTime := metav1.Now()
		updatedConfig.Status.Status = StatusCompleted
		updatedConfig.Status.Completed = &completedTime
		logger.Info("all active providers have contributed, marking as completed", "UUID", updatedConfig.Spec.UUID)
	} else {
		logger.Info("waiting for more providers to contribute",
			"needed", activeProviderCount,
			"current", contributorCount)
	}

	err = r.Status().Update(ctx, updatedConfig)
	if err != nil {
		if errors.IsConflict(err) {
			logger.Info("conflict updating status, will retry", "UUID", updatedConfig.Spec.UUID)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "failed to update KrknOperatorTargetProviderConfig status")
		return ctrl.Result{}, err
	}

	logger.Info("successfully updated provider config", "UUID", updatedConfig.Spec.UUID, "operator-name", r.OperatorName, "status", updatedConfig.Status.Status)

	// Cleanup old KrknOperatorTargetProviderConfig resources
	deletedCount, err := provider.CleanupOldResources(
		ctx,
		r.Client,
		&krknv1alpha1.KrknOperatorTargetProviderConfigList{},
		r.OperatorNamespace,
		CleanupThresholdSeconds,
		func(obj client.Object) *metav1.Time {
			config := obj.(*krknv1alpha1.KrknOperatorTargetProviderConfig)
			return config.Status.Created
		},
	)
	if err != nil {
		logger.Error(err, "failed to cleanup old KrknOperatorTargetProviderConfig resources")
		// Don't fail the reconciliation due to cleanup errors
	} else if deletedCount > 0 {
		logger.Info("cleaned up old KrknOperatorTargetProviderConfig resources", "count", deletedCount)
	}

	return ctrl.Result{}, nil
}

// buildConfigSchema constructs the JSON schema for the provider configuration
// based on available ACM managed clusters and their secrets
func (r *KrknOperatorTargetProviderConfigReconciler) buildConfigSchema(ctx context.Context) (string, error) {
	logger := log.FromContext(ctx)

	// Get all managed clusters
	managedClusters, err := r.getManagedClusters(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get managed clusters: %w", err)
	}

	logger.Info("found managed clusters for config schema", "count", len(managedClusters.Items))

	// Build input fields for each cluster
	var fields []typing.InputField

	// Create group definitions
	secretGroupName := "ACM_SECRET_GROUP"
	secretGroupShortDesc := "ACM/OCM Secret Selection"
	secretGroupDesc := "Select secrets for direct API connection to managed clusters. Each secret contains the CA certificate and service account token for cluster authentication."

	secretGroupField := typing.InputField{
		Name:             &secretGroupName,
		ShortDescription: &secretGroupShortDesc,
		Description:      &secretGroupDesc,
		Variable:         &secretGroupName,
		Type:             typing.Group,
		Required:         false,
		Secret:           false,
	}
	fields = append(fields, secretGroupField)

	proxyGroupName := "ACM_USE_PROXY_GROUP"
	proxyGroupShortDesc := "Cluster Proxy Configuration"
	proxyGroupDesc := "Enable cluster proxy connection for managed clusters. Note: Activating proxy for a cluster overrides the secret selection and uses the cluster-proxy-addon for communication. To return to direct API access, disable the proxy for that cluster. Requires cluster-proxy-addon and ManifestWork to be deployed."

	proxyGroupField := typing.InputField{
		Name:             &proxyGroupName,
		ShortDescription: &proxyGroupShortDesc,
		Description:      &proxyGroupDesc,
		Variable:         &proxyGroupName,
		Type:             typing.Group,
		Required:         false,
		Secret:           false,
	}
	fields = append(fields, proxyGroupField)

	for _, cluster := range managedClusters.Items {
		clusterName := cluster.Metadata.Name

		// List secrets in the cluster namespace
		secrets, err := r.listSecretsInNamespace(ctx, clusterName)
		if err != nil {
			logger.Error(err, "failed to list secrets in namespace, skipping cluster", "cluster", clusterName)
			continue
		}

		if len(secrets) == 0 {
			logger.Info("no secrets found in cluster namespace, skipping", "cluster", clusterName)
			continue
		}

		// Sort secrets alphabetically
		sort.Strings(secrets)

		// Create variable name from namespace
		varName := formatNamespaceToVarName(clusterName)
		defaultSecret := getDefaultSecret(clusterName, secrets)
		shortDesc := fmt.Sprintf("Secret for %s", clusterName)
		description := fmt.Sprintf("Select the secret to use for cluster %s authentication. Available secrets: %s",
			clusterName, strings.Join(secrets, ", "))
		separator := ","
		allowedValues := strings.Join(secrets, ",")

		// Build the InputField using typing package
		// Note: Secret fields do NOT mutually exclude proxy fields
		// Only proxy fields mutually exclude secrets (unidirectional)
		field := typing.InputField{
			Name:             &varName,
			ShortDescription: &shortDesc,
			Description:      &description,
			Variable:         &varName,
			Type:             typing.Enum,
			Default:          &defaultSecret,
			Separator:        &separator,
			AllowedValues:    &allowedValues,
			Required:         false,
			Secret:           false,
			Group:            &secretGroupName,
		}

		fields = append(fields, field)
		logger.Info("added config field for cluster", "cluster", clusterName, "variable", varName, "secret-count", len(secrets))
	}

	// Add proxy mode toggle for each cluster
	for _, cluster := range managedClusters.Items {
		clusterName := cluster.Metadata.Name

		// Create proxy mode toggle field
		proxyVarName := formatProxyVarName(clusterName)
		secretVarName := formatNamespaceToVarName(clusterName)
		proxyShortDesc := fmt.Sprintf("Proxy mode for %s", clusterName)
		proxyDescription := fmt.Sprintf("Enable cluster proxy connection for %s instead of direct API access. "+
			"Requires cluster-proxy-addon and ManifestWork '%s' to be deployed.",
			clusterName, ManifestWorkName)
		proxyDefaultValue := getDefaultProxyMode(clusterName)
		proxySeparator := ","
		proxyAllowedValues := "true,false"

		proxyField := typing.InputField{
			Name:             &proxyVarName,
			ShortDescription: &proxyShortDesc,
			Description:      &proxyDescription,
			Variable:         &proxyVarName,
			Type:             typing.Enum,
			Default:          &proxyDefaultValue,
			Separator:        &proxySeparator,
			AllowedValues:    &proxyAllowedValues,
			Required:         false,
			Secret:           false,
			Group:            &proxyGroupName,
			MutuallyExcludes: &secretVarName,
		}

		fields = append(fields, proxyField)
		logger.Info("added proxy config field for cluster",
			"cluster", clusterName,
			"variable", proxyVarName)
	}

	// Serialize fields to JSON using InputField.MarshalJSON
	// which preserves the format expected by the parser
	var jsonFields []json.RawMessage
	for _, field := range fields {
		fieldJSON, err := field.MarshalJSON()
		if err != nil {
			return "", fmt.Errorf("failed to marshal field %s: %w", *field.Name, err)
		}
		jsonFields = append(jsonFields, fieldJSON)
	}

	// Marshal the array of raw JSON messages
	jsonData, err := json.Marshal(jsonFields)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config schema: %w", err)
	}

	return string(jsonData), nil
}

// formatNamespaceToVarName formats a namespace name to a valid environment variable name
// Converts to uppercase and replaces hyphens with underscores
// Prefixes with ACM_SECRET_
func formatNamespaceToVarName(namespace string) string {
	// Replace hyphens with underscores
	formatted := strings.ReplaceAll(namespace, "-", "_")
	// Convert to uppercase
	formatted = strings.ToUpper(formatted)
	// Add prefix
	return "ACM_SECRET_" + formatted
}

// listSecretsInNamespace returns a list of secret names in the given namespace
// that contain both "ca.crt" and "token" keys in their Data field
func (r *KrknOperatorTargetProviderConfigReconciler) listSecretsInNamespace(ctx context.Context, namespace string) ([]string, error) {
	secretList := &corev1.SecretList{}
	err := r.List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets in namespace %s: %w", namespace, err)
	}

	secretNames := make([]string, 0, len(secretList.Items))
	for _, secret := range secretList.Items {
		// Only include secrets that have both ca.crt and token keys
		if hasRequiredKeys(secret.Data) {
			secretNames = append(secretNames, secret.Name)
		}
	}

	return secretNames, nil
}

// hasRequiredKeys checks if a secret contains both "ca.crt" and "token" keys
func hasRequiredKeys(data map[string][]byte) bool {
	_, hasCert := data["ca.crt"]
	_, hasToken := data["token"]
	return hasCert && hasToken
}

// getDefaultProxyMode determines the default proxy mode from configstore
// Returns the current value from configstore, or "false" if not set
func getDefaultProxyMode(clusterName string) string {
	// Get the configstore singleton
	store := kvstore.Get()

	// Check if there's a configured value in configstore
	varName := formatProxyVarName(clusterName)
	if configuredValue, ok := store.GetValue(varName); ok && configuredValue != "" {
		// Normalize value to "true" or "false"
		normalized := strings.ToLower(strings.TrimSpace(configuredValue))
		if normalized == "true" || normalized == "yes" || normalized == "1" {
			return "true"
		}
		if normalized == "false" || normalized == "no" || normalized == "0" {
			return "false"
		}
		// Invalid value, return default
		return "false"
	}

	// No configured value, return default
	return "false"
}

// getDefaultSecret determines the default secret from a list
// Priority order:
// 1. Value from configstore for ACM_SECRET_<NAMESPACE> (if exists, valid, and DIFFERENT from ACMDefaultSecret)
// 2. "application-manager" (if present in the list)
// 3. First secret in the list
func getDefaultSecret(namespace string, secrets []string) string {
	if len(secrets) == 0 {
		return ""
	}

	// Get the configstore singleton
	store := kvstore.Get()

	// Check if there's a configured value in configstore
	varName := formatNamespaceToVarName(namespace)
	if configuredValue, ok := store.GetValue(varName); ok && configuredValue != "" {
		// Only use configstore value if it's DIFFERENT from ACMDefaultSecret
		if configuredValue != ACMDefaultSecret {
			// Verify the configured value is in the available secrets list
			for _, secret := range secrets {
				if secret == configuredValue {
					return configuredValue
				}
			}
		}
		// If configured value is ACMDefaultSecret or not in the list, fall through to next priority
	}

	// Check if ACMDefaultSecret is in the list
	for _, secret := range secrets {
		if secret == ACMDefaultSecret {
			return ACMDefaultSecret
		}
	}

	// Return the first secret if application-manager not found
	return secrets[0]
}

// getManagedClusters retrieves all managed clusters from ACM
func (r *KrknOperatorTargetProviderConfigReconciler) getManagedClusters(ctx context.Context) (*ManagedClusterList, error) {
	// Create an unstructured list to fetch managed clusters
	clusterList := &unstructured.UnstructuredList{}
	clusterList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cluster.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManagedClusterList",
	})

	// List all managed clusters using the generic client
	err := r.List(ctx, clusterList)
	if err != nil {
		return nil, fmt.Errorf("failed to list managed clusters: %w", err)
	}

	// Convert the unstructured result to our struct
	managedClusterList := &ManagedClusterList{}
	data, err := json.Marshal(clusterList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal managed clusters: %w", err)
	}

	err = json.Unmarshal(data, managedClusterList)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal managed clusters: %w", err)
	}

	return managedClusterList, nil
}

// getConfigDataKeys returns a slice of keys from the ConfigData map
func getConfigDataKeys(m map[string]krknv1alpha1.ProviderConfigData) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// SetupWithManager sets up the controller with the Manager.
func (r *KrknOperatorTargetProviderConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&krknv1alpha1.KrknOperatorTargetProviderConfig{}).
		Named("krknoperatortargetproviderconfig").
		Complete(r)
}
