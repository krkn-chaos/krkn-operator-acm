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
	"sigs.k8s.io/controller-runtime/pkg/log"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator-acm/api/v1alpha1"
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
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krkntargetrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krkntargetrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=krkn.krkn-chaos.dev,resources=krkntargetrequests/finalizers,verbs=update
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
			logger.Info("KrknTargetRequest resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get KrknTargetRequest")
		return ctrl.Result{}, err
	}

	// Initialize status to "pending" if it's empty (new resource)
	if krknRequest.Status.Status == "" {
		logger.Info("Initializing status to pending", "UUID", krknRequest.Spec.UUID)
		krknRequest.Status.Status = "pending"
		err = r.Status().Update(ctx, krknRequest)
		if err != nil {
			logger.Error(err, "Failed to initialize status")
			return ctrl.Result{}, err
		}
		// Requeue to process in the next reconciliation
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if the request is already completed
	if krknRequest.Status.Status == "Completed" {
		logger.Info("KrknTargetRequest already completed", "UUID", krknRequest.Spec.UUID)
		return ctrl.Result{}, nil
	}

	// Check if status is pending
	if krknRequest.Status.Status != "pending" {
		logger.Info("KrknTargetRequest status is not pending, skipping", "status", krknRequest.Status.Status)
		return ctrl.Result{}, nil
	}

	logger.Info("Processing KrknTargetRequest", "UUID", krknRequest.Spec.UUID)

	// Get managed clusters
	managedClusters, err := r.getManagedClusters(ctx)
	if err != nil {
		logger.Error(err, "Failed to get managed clusters")
		return ctrl.Result{}, err
	}

	logger.Info("Found managed clusters", "count", len(managedClusters.Items))

	// Collect cluster data
	clustersData := make(map[string]ClusterData)
	var targetData []krknv1alpha1.ClusterTarget

	for _, cluster := range managedClusters.Items {
		clusterName := cluster.Metadata.Name
		logger.Info("Processing cluster", "name", clusterName)

		if len(cluster.Spec.ManagedClusterClientConfigs) == 0 {
			logger.Info("Cluster has no client configs, skipping", "name", clusterName)
			continue
		}

		clusterURL := cluster.Spec.ManagedClusterClientConfigs[0].URL
		clusterCABundle := cluster.Spec.ManagedClusterClientConfigs[0].CABundle

		// Get the application-manager secret from the cluster namespace
		secret, err := r.getApplicationManagerSecret(ctx, clusterName)
		if err != nil {
			logger.Error(err, "Failed to get application-manager secret", "cluster", clusterName)
			continue
		}

		// Extract ca.crt and token from secret
		caCrt, ok := secret.Data["ca.crt"]
		if !ok {
			logger.Error(fmt.Errorf("ca.crt not found in secret"), "Missing ca.crt", "cluster", clusterName)
			continue
		}

		token, ok := secret.Data["token"]
		if !ok {
			logger.Error(fmt.Errorf("token not found in secret"), "Missing token", "cluster", clusterName)
			continue
		}

		// Generate kubeconfig
		kubeconfig, err := r.generateKubeconfig(clusterName, clusterURL, clusterCABundle, string(caCrt), string(token))
		if err != nil {
			logger.Error(err, "Failed to generate kubeconfig", "cluster", clusterName)
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
	err = r.createManagedClustersSecret(ctx, krknRequest.Spec.UUID, clustersData)
	if err != nil {
		logger.Error(err, "Failed to create managed clusters secret")
		return ctrl.Result{}, err
	}

	logger.Info("Created secret with managed clusters data", "secretName", krknRequest.Spec.UUID)

	// Update KrknTargetRequest status
	krknRequest.Status.Status = "Completed"
	krknRequest.Status.TargetData = targetData

	err = r.Status().Update(ctx, krknRequest)
	if err != nil {
		logger.Error(err, "Failed to update KrknTargetRequest status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully completed KrknTargetRequest", "UUID", krknRequest.Spec.UUID)
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

// getApplicationManagerSecret retrieves the application-manager secret from a cluster namespace
func (r *KrknTargetRequestReconciler) getApplicationManagerSecret(ctx context.Context, clusterName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "application-manager",
		Namespace: clusterName,
	}, secret)

	if err != nil {
		return nil, fmt.Errorf("failed to get application-manager secret in namespace %s: %w", clusterName, err)
	}

	return secret, nil
}

// generateKubeconfig generates a kubeconfig for a managed cluster
func (r *KrknTargetRequestReconciler) generateKubeconfig(clusterName, clusterURL, clusterCABundle, caCrt, token string) (string, error) {
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

// createManagedClustersSecret creates a secret containing all managed clusters data
func (r *KrknTargetRequestReconciler) createManagedClustersSecret(ctx context.Context, uuid string, clustersData map[string]ClusterData) error {
	// Convert clusters data to JSON
	jsonData, err := json.Marshal(clustersData)
	if err != nil {
		return fmt.Errorf("failed to marshal clusters data: %w", err)
	}

	// Create the secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"managed-clusters": jsonData,
		},
	}

	// Check if secret already exists
	existingSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: uuid, Namespace: "default"}, existingSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new secret
			err = r.Create(ctx, secret)
			if err != nil {
				return fmt.Errorf("failed to create secret: %w", err)
			}
		} else {
			return fmt.Errorf("failed to check if secret exists: %w", err)
		}
	} else {
		// Update existing secret
		existingSecret.Data = secret.Data
		err = r.Update(ctx, existingSecret)
		if err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KrknTargetRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&krknv1alpha1.KrknTargetRequest{}).
		Named("krkntargetrequest").
		Complete(r)
}
