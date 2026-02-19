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
		DefaultConfigMapName,
		jsonSchema,
		r.OperatorNamespace,
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

	logger.Info("successfully updated provider config", "UUID", freshConfig.Spec.UUID, "operator-name", r.OperatorName)

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

		// Build the InputField
		field := typing.InputField{
			Name:             &varName,
			ShortDescription: &shortDesc,
			Description:      &description,
			Variable:         &varName,
			Type:             typing.Enum,
			Default:          &defaultSecret,
			Separator:        &separator,
			AllowedValues:    &allowedValues,
			Required:         true,
			Secret:           false,
		}

		fields = append(fields, field)
		logger.Info("added config field for cluster", "cluster", clusterName, "variable", varName, "secret-count", len(secrets))
	}

	// Serialize fields to JSON
	jsonData, err := json.Marshal(fields)
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

// getDefaultSecret determines the default secret from a list
// Priority order:
// 1. Value from configstore for ACM_SECRET_<NAMESPACE> (if exists and valid)
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
		// Verify the configured value is in the available secrets list
		for _, secret := range secrets {
			if secret == configuredValue {
				return configuredValue
			}
		}
		// If configured value is not in the list, fall through to next priority
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

// SetupWithManager sets up the controller with the Manager.
func (r *KrknOperatorTargetProviderConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&krknv1alpha1.KrknOperatorTargetProviderConfig{}).
		Named("krknoperatortargetproviderconfig").
		Complete(r)
}
