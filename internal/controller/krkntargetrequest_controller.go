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
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
	"github.com/krkn-chaos/krkn-operator/pkg/provider"
)

// ManagedCluster represents an ACM managed cluster
type ManagedCluster struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		ManagedClusterClientConfigs []struct {
			URL      string `json:"url"`
			CABundle string `json:"caBundle"`
		} `json:"managedClusterClientConfigs"`
	} `json:"spec"`
}

// ManagedClusterList represents a list of managed clusters
type ManagedClusterList struct {
	Items []ManagedCluster `json:"items"`
}

// ClusterData represents the data stored for each cluster
type ClusterData struct {
	ClusterName string `json:"cluster-name"`
	ClusterAPI  string `json:"cluster-api"`
	Kubeconfig  string `json:"kubeconfig"`
}

// KrknTargetRequestReconciler reconciles a KrknTargetRequest object
type KrknTargetRequestReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	OperatorName      string
	OperatorNamespace string
}

// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krkntargetrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krkntargetrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krkntargetrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krknoperatortargetproviders,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KrknTargetRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the KrknTargetRequest instance
	krknRequest := &krknv1alpha1.KrknTargetRequest{}
	err := r.Get(ctx, req.NamespacedName, krknRequest)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("krknTargetRequest resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get KrknTargetRequest")
		return ctrl.Result{}, err
	}

	// Initialize status to "pending" if it's empty (new resource)
	if krknRequest.Status.Status == "" {
		logger.Info("initializing status to pending", "UUID", krknRequest.Spec.UUID)

		// Add UUID label if not present
		if krknRequest.Labels == nil {
			krknRequest.Labels = make(map[string]string)
		}

		needsLabelUpdate := false
		if _, exists := krknRequest.Labels["krkn.krkn-chaos.dev/uuid"]; !exists {
			krknRequest.Labels["krkn.krkn-chaos.dev/uuid"] = krknRequest.Spec.UUID
			needsLabelUpdate = true
		}

		// Update labels if needed
		if needsLabelUpdate {
			err = r.Update(ctx, krknRequest)
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
		krknRequest.Status.Status = "pending"
		err = r.Status().Update(ctx, krknRequest)
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

	// Check if the request is already completed
	if krknRequest.Status.Status == StatusCompleted {
		logger.Info("krknTargetRequest already completed", "UUID", krknRequest.Spec.UUID)
		return ctrl.Result{}, nil
	}

	// Check if status is pending
	if krknRequest.Status.Status != "pending" {
		logger.Info("krknTargetRequest status is not pending, skipping", "status", krknRequest.Status.Status)
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

	logger.Info("processing krknTargetRequest", "UUID", krknRequest.Spec.UUID)

	// Get managed clusters
	managedClusters, err := r.getManagedClusters(ctx)
	if err != nil {
		logger.Error(err, "failed to get managed clusters")
		return ctrl.Result{}, err
	}

	logger.Info("found managed clusters", "count", len(managedClusters.Items))

	// Collect cluster data
	clustersData := make(map[string]ClusterData)
	targetData := make([]krknv1alpha1.ClusterTarget, 0, len(managedClusters.Items))

	for _, cluster := range managedClusters.Items {
		clusterName := cluster.Metadata.Name
		logger.Info("processing cluster", "name", clusterName)

		if len(cluster.Spec.ManagedClusterClientConfigs) == 0 {
			logger.Info("cluster has no client configs, skipping", "name", clusterName)
			continue
		}

		clusterURL := cluster.Spec.ManagedClusterClientConfigs[0].URL
		clusterCABundle := cluster.Spec.ManagedClusterClientConfigs[0].CABundle

		// Get the configured secret from configstore, or use default
		secretName := r.getConfiguredSecretName(ctx, clusterName)
		secret, err := r.getClusterSecret(ctx, clusterName, secretName)
		if err != nil {
			logger.Error(err, "failed to get cluster secret", "cluster", clusterName, "secret", secretName)
			continue
		}

		// Extract token from secret
		token, ok := secret.Data["token"]
		if !ok {
			logger.Error(fmt.Errorf("token not found in secret"), "missing token", "cluster", clusterName)
			continue
		}

		// Generate kubeconfig
		kubeconfig, err := r.generateKubeconfig(clusterName, clusterURL, clusterCABundle, string(token))
		if err != nil {
			logger.Error(err, "failed to generate kubeconfig", "cluster", clusterName)
			continue
		}

		// Encode kubeconfig as base64
		kubeconfigBase64 := base64.StdEncoding.EncodeToString([]byte(kubeconfig))

		clustersData[clusterName] = ClusterData{
			ClusterName: clusterName,
			ClusterAPI:  clusterURL,
			Kubeconfig:  kubeconfigBase64,
		}

		targetData = append(targetData, krknv1alpha1.ClusterTarget{
			ClusterName:   clusterName,
			ClusterAPIURL: clusterURL,
		})
	}

	// Create secret with managed clusters data
	err = r.createManagedClustersSecret(ctx, krknRequest, clustersData)
	if err != nil {
		logger.Error(err, "failed to create managed clusters secret")
		return ctrl.Result{}, err
	}

	logger.Info("created secret with managed clusters data", "secretName", krknRequest.Spec.UUID)

	// Initialize TargetData map if nil
	if krknRequest.Status.TargetData == nil {
		krknRequest.Status.TargetData = make(map[string][]krknv1alpha1.ClusterTarget)
	}

	// Set target data for this operator
	krknRequest.Status.TargetData[r.OperatorName] = targetData

	logger.Info("set target data for operator", "operator-name", r.OperatorName, "target-count", len(targetData))

	// Count only active providers (providerList was already fetched earlier)
	activeProviderCount := countActiveProviders(providerList)
	contributorCount := len(krknRequest.Status.TargetData)

	logger.Info("provider status",
		"total-providers", len(providerList.Items),
		"active-providers", activeProviderCount,
		"contributors", contributorCount,
		"contributor-names", getMapKeys(krknRequest.Status.TargetData),
		"provider-namespaces", getProviderNamespaces(providerList.Items))

	// Check if all active providers have contributed
	if shouldMarkAsCompleted(activeProviderCount, contributorCount) {
		completedTime := metav1.Now()
		krknRequest.Status.Status = StatusCompleted
		krknRequest.Status.Completed = &completedTime
		logger.Info("all active providers have contributed, marking as completed", "UUID", krknRequest.Spec.UUID)
	} else {
		logger.Info("waiting for more providers to contribute",
			"needed", activeProviderCount,
			"current", contributorCount)
	}

	err = r.Status().Update(ctx, krknRequest)
	if err != nil {
		if errors.IsConflict(err) {
			logger.Info("conflict updating status, will retry", "UUID", krknRequest.Spec.UUID)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "failed to update KrknTargetRequest status")
		return ctrl.Result{}, err
	}

	logger.Info("successfully updated krknTargetRequest", "UUID", krknRequest.Spec.UUID, "status", krknRequest.Status.Status)

	// Cleanup old KrknTargetRequest resources
	deletedCount, err := provider.CleanupOldResources(
		ctx,
		r.Client,
		&krknv1alpha1.KrknTargetRequestList{},
		r.OperatorNamespace,
		CleanupThresholdSeconds,
		func(obj client.Object) *metav1.Time {
			// Use CreationTimestamp from metadata since Status.Created was removed
			ts := obj.GetCreationTimestamp()
			return &ts
		},
	)
	if err != nil {
		logger.Error(err, "failed to cleanup old KrknTargetRequest resources")
		// Don't fail the reconciliation due to cleanup errors
	} else if deletedCount > 0 {
		logger.Info("cleaned up old KrknTargetRequest resources", "count", deletedCount)
	}

	return ctrl.Result{}, nil
}

// getManagedClusters retrieves all managed clusters from ACM
func (r *KrknTargetRequestReconciler) getManagedClusters(ctx context.Context) (*ManagedClusterList, error) {
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

// getConfiguredSecretName determines which secret to use for a cluster
// by querying the configstore for ACM_SECRET_<CLUSTER_NAME>
// Falls back to ACMDefaultSecret if not configured
func (r *KrknTargetRequestReconciler) getConfiguredSecretName(ctx context.Context, clusterName string) string {
	logger := log.FromContext(ctx)
	store := kvstore.Get()

	// Normalize cluster name to variable format
	varName := formatNamespaceToVarName(clusterName)
	sn, mimm := store.GetValue(varName)
	fmt.Sprintf("%s %m", sn, mimm)
	// Query configstore
	if secretName, ok := store.GetValue(varName); ok && secretName != "" {
		logger.Info("using configured secret for cluster",
			"cluster", clusterName,
			"secret", secretName,
			"config-key", varName)
		return secretName
	}

	// Fallback to default
	logger.Info("using default secret for cluster",
		"cluster", clusterName,
		"secret", ACMDefaultSecret)
	return ACMDefaultSecret
}

// getClusterSecret retrieves a secret from a cluster namespace
func (r *KrknTargetRequestReconciler) getClusterSecret(ctx context.Context, clusterName, secretName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: clusterName,
	}, secret)

	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, clusterName, err)
	}

	return secret, nil
}

// generateKubeconfig generates a kubeconfig for a managed cluster
func (r *KrknTargetRequestReconciler) generateKubeconfig(clusterName, clusterURL, clusterCABundle, token string) (string, error) {
	// Use the CA bundle from the managed cluster spec
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
users:
- name: %s
  user:
    token: %s
`, clusterCABundle, clusterURL, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, token)

	return kubeconfig, nil
}

// createManagedClustersSecret creates or updates a secret containing managed clusters data
// The secret format is: map[operator-name]map[cluster-name]ClusterData
// This allows multiple operators to contribute their cluster data without conflicts
func (r *KrknTargetRequestReconciler) createManagedClustersSecret(ctx context.Context, krknRequest *krknv1alpha1.KrknTargetRequest, clustersData map[string]ClusterData) error {
	secretNamespace := r.OperatorNamespace
	secretName := krknRequest.Spec.UUID

	// Check if secret already exists
	existingSecret := &corev1.Secret{}
	getErr := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, existingSecret)

	var allOperatorData map[string]map[string]ClusterData
	secretExists := getErr == nil

	if getErr != nil && !errors.IsNotFound(getErr) {
		return fmt.Errorf("failed to check if secret exists: %w", getErr)
	}

	if secretExists {
		// Secret exists, parse existing data
		existingData, ok := existingSecret.Data["managed-clusters"]
		if ok && len(existingData) > 0 {
			// Try to unmarshal as multi-operator format first
			err := json.Unmarshal(existingData, &allOperatorData)
			if err != nil {
				// If that fails, try old format and migrate it
				var oldFormatData map[string]ClusterData
				err = json.Unmarshal(existingData, &oldFormatData)
				if err != nil {
					return fmt.Errorf("failed to unmarshal existing secret data: %w", err)
				}
				// Migrate old format: assume it came from an operator with default name
				allOperatorData = map[string]map[string]ClusterData{
					"krkn-operator-acm": oldFormatData,
				}
			}
		} else {
			allOperatorData = make(map[string]map[string]ClusterData)
		}
	} else {
		// Secret doesn't exist, create new structure
		allOperatorData = make(map[string]map[string]ClusterData)
	}

	// Add/update this operator's data
	allOperatorData[r.OperatorName] = clustersData

	// Marshal the complete multi-operator structure
	jsonData, err := json.Marshal(allOperatorData)
	if err != nil {
		return fmt.Errorf("failed to marshal clusters data: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{
			"managed-clusters": jsonData,
		},
	}

	// Set owner reference to enable automatic cleanup when KrknTargetRequest is deleted
	if err := ctrl.SetControllerReference(krknRequest, secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on secret: %w", err)
	}

	if secretExists {
		// Update existing secret
		existingSecret.Data = secret.Data
		existingSecret.ObjectMeta.OwnerReferences = secret.ObjectMeta.OwnerReferences
		err = r.Update(ctx, existingSecret)
		if err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}
	} else {
		// Create new secret
		err = r.Create(ctx, secret)
		if err != nil {
			// Handle race condition: secret was created between check and create
			if errors.IsAlreadyExists(err) {
				// Fetch the now-existing secret and update it
				if getErr := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, existingSecret); getErr != nil {
					return fmt.Errorf("failed to get secret after AlreadyExists: %w", getErr)
				}
				existingSecret.Data = secret.Data
				existingSecret.ObjectMeta.OwnerReferences = secret.ObjectMeta.OwnerReferences
				if updateErr := r.Update(ctx, existingSecret); updateErr != nil {
					return fmt.Errorf("failed to update secret after AlreadyExists: %w", updateErr)
				}
			} else {
				return fmt.Errorf("failed to create secret: %w", err)
			}
		}
	}

	return nil
}

// getMapKeys returns a slice of keys from a map
func getMapKeys(m map[string][]krknv1alpha1.ClusterTarget) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getProviderNamespaces returns a list of namespaces where providers are registered
func getProviderNamespaces(providers []krknv1alpha1.KrknOperatorTargetProvider) []string {
	namespaces := make([]string, 0, len(providers))
	for _, p := range providers {
		namespaces = append(namespaces, p.Namespace)
	}
	return namespaces
}

// NewNamespaceFilter creates a predicate that only allows events from a specific namespace
func NewNamespaceFilter(namespace string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetNamespace() == namespace
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetNamespace() == namespace
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetNamespace() == namespace
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return e.Object.GetNamespace() == namespace
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *KrknTargetRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&krknv1alpha1.KrknTargetRequest{}).
		Named("krkntargetrequest").
		WithEventFilter(NewNamespaceFilter(r.OperatorNamespace)).
		Complete(r)
}
