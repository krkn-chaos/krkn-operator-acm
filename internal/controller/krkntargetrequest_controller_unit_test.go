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
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	krknv1alpha1 "github.com/krkn-chaos/krkn-operator/api/v1alpha1"
	kvstore "github.com/krkn-chaos/krkn-operator/pkg/configstore"
)

func TestBuildClusterTargetWithLiveness(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantOnline bool
	}{
		{name: "online cluster", statusCode: http.StatusOK, wantOnline: true},
		{name: "offline cluster", statusCode: http.StatusServiceUnavailable, wantOnline: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			target, err := buildClusterTargetWithLiveness(
				context.Background(),
				"managed-cluster",
				server.URL,
				testLivenessKubeconfig(t, server.URL),
			)

			if tt.wantOnline && err != nil {
				t.Fatalf("buildClusterTargetWithLiveness() error = %v, want nil", err)
			}
			if !tt.wantOnline && err == nil {
				t.Fatal("buildClusterTargetWithLiveness() error = nil, want liveness error")
			}

			if target.Online == nil {
				t.Fatal("Online is nil, want a liveness result")
			}
			if *target.Online != tt.wantOnline {
				t.Fatalf("Online = %v, want %v", *target.Online, tt.wantOnline)
			}
			if target.CheckedAt == nil || target.CheckedAt.IsZero() {
				t.Fatal("CheckedAt is nil or zero")
			}
			if target.ClusterName != "managed-cluster" || target.ClusterAPIURL != server.URL {
				t.Fatalf("target identity = %+v", target)
			}
		})
	}
}

func TestOfflineClusterTarget(t *testing.T) {
	target := offlineClusterTarget("offline-cluster", "https://offline.example.com:6443")

	if target.ClusterName != "offline-cluster" {
		t.Fatalf("ClusterName = %q, want %q", target.ClusterName, "offline-cluster")
	}
	if target.ClusterAPIURL != "https://offline.example.com:6443" {
		t.Fatalf("ClusterAPIURL = %q, want %q", target.ClusterAPIURL, "https://offline.example.com:6443")
	}
	if target.Online != nil {
		t.Fatal("Online should be omitted when no liveness check was performed")
	}
	if target.CheckedAt != nil {
		t.Fatal("CheckedAt should be omitted when no liveness check was performed")
	}
}

func TestClusterHealthStatus(t *testing.T) {
	tests := []struct {
		name       string
		conditions []ManagedClusterCondition
		want       krknv1alpha1.ClusterHealthStatus
	}{
		{
			name:       "available",
			conditions: []ManagedClusterCondition{{Type: managedClusterConditionAvailable, Status: "True"}},
			want:       krknv1alpha1.ClusterStatusHealthy,
		},
		{
			name:       "not available",
			conditions: []ManagedClusterCondition{{Type: managedClusterConditionAvailable, Status: "False"}},
			want:       krknv1alpha1.ClusterStatusUnhealthy,
		},
		{
			name: "stopped lease updates",
			conditions: []ManagedClusterCondition{{
				Type: managedClusterConditionAvailable, Status: "Unknown", Reason: "ManagedClusterLeaseUpdateStopped",
			}},
			want: krknv1alpha1.ClusterStatusUnhealthy,
		},
		{
			name:       "other unknown reason",
			conditions: []ManagedClusterCondition{{Type: managedClusterConditionAvailable, Status: "Unknown", Reason: "Initializing"}},
			want:       krknv1alpha1.ClusterStatusUnknown,
		},
		{
			name:       "available condition missing",
			conditions: []ManagedClusterCondition{{Type: "Managed", Status: "True"}},
			want:       krknv1alpha1.ClusterStatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := ManagedCluster{}
			cluster.Status.Conditions = tt.conditions
			if got := clusterHealthStatus(cluster); got != tt.want {
				t.Fatalf("clusterHealthStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestManagedClusterAvailableConditionDecodesFromACMJSON(t *testing.T) {
	const payload = `{"metadata":{"name":"local-cluster"},"status":{"conditions":[{"type":"ManagedClusterConditionAvailable","status":"True","reason":"ManagedClusterAvailable"}]}}`

	var cluster ManagedCluster
	if err := json.Unmarshal([]byte(payload), &cluster); err != nil {
		t.Fatalf("failed to decode ManagedCluster status: %v", err)
	}
	if got := clusterHealthStatus(cluster); got != krknv1alpha1.ClusterStatusHealthy {
		t.Fatalf("clusterHealthStatus() = %q, want %q", got, krknv1alpha1.ClusterStatusHealthy)
	}
}

func TestProviderContributionIsStable(t *testing.T) {
	target := offlineClusterTarget("offline-cluster", "https://offline.example.com:6443")
	request := krknv1alpha1.KrknTargetRequestStatus{
		TargetData: map[string][]krknv1alpha1.ClusterTarget{
			"krkn-operator-acm": {target},
		},
	}

	if _, contributed := request.TargetData["krkn-operator-acm"]; !contributed {
		t.Fatal("expected ACM provider contribution to be detected")
	}
}

func TestReconcileCompletesExistingContribution(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := krknv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add KrknOperator scheme: %v", err)
	}

	now := metav1.Now()
	request := &krknv1alpha1.KrknTargetRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "target-request",
			Namespace:         "operator-system",
			CreationTimestamp: now,
		},
		Spec: krknv1alpha1.KrknTargetRequestSpec{UUID: "target-request-uuid"},
		Status: krknv1alpha1.KrknTargetRequestStatus{
			Status: "pending",
			TargetData: map[string][]krknv1alpha1.ClusterTarget{
				"krkn-operator-acm": {offlineClusterTarget("cluster", "https://api.example.com")},
			},
		},
	}
	providerObject := &krknv1alpha1.KrknOperatorTargetProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "acm-provider",
			Namespace:         "operator-system",
			CreationTimestamp: now,
		},
		Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{
			OperatorName: "krkn-operator-acm",
			Active:       true,
		},
	}

	reconciler := &KrknTargetRequestReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(request).
			WithObjects(request, providerObject).
			Build(),
		Scheme:            scheme,
		OperatorName:      "krkn-operator-acm",
		OperatorNamespace: "operator-system",
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: request.Name, Namespace: request.Namespace},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &krknv1alpha1.KrknTargetRequest{}
	if err := reconciler.Get(context.Background(), types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, updated); err != nil {
		t.Fatalf("failed to get reconciled request: %v", err)
	}
	if updated.Status.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", updated.Status.Status, StatusCompleted)
	}
	if updated.Status.Completed == nil {
		t.Fatal("Completed timestamp is nil")
	}
}

func TestReconcileMarksProxyClusterUnhealthyWhenManifestWorkIsNotReady(t *testing.T) {
	ctx := context.Background()
	const clusterName = "managed-cluster"
	const proxyURL = "https://cluster-proxy-addon-user.multicluster-engine.svc:9092/managed-cluster"

	store := kvstore.Get()
	proxyConfigKey := formatProxyVarName(clusterName)
	store.SetValue(proxyConfigKey, "true")
	t.Cleanup(func() { store.Delete(proxyConfigKey) })

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add core scheme: %v", err)
	}
	if err := krknv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add KrknOperator scheme: %v", err)
	}
	managedClusterGVK := schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	}
	manifestWorkGVK := schema.GroupVersionKind{
		Group: "work.open-cluster-management.io", Version: "v1", Kind: "ManifestWork",
	}
	proxyConfigGVK := schema.GroupVersionKind{
		Group: "proxy.open-cluster-management.io", Version: "v1alpha1", Kind: "ManagedProxyConfiguration",
	}
	for _, gvk := range []schema.GroupVersionKind{managedClusterGVK, manifestWorkGVK, proxyConfigGVK} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
	}

	now := metav1.Now()
	request := &krknv1alpha1.KrknTargetRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "target-request", Namespace: "operator-system", CreationTimestamp: now,
		},
		Spec:   krknv1alpha1.KrknTargetRequestSpec{UUID: "target-request-uuid"},
		Status: krknv1alpha1.KrknTargetRequestStatus{Status: "pending"},
	}
	providerObject := &krknv1alpha1.KrknOperatorTargetProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "acm-provider", Namespace: "operator-system", CreationTimestamp: now,
		},
		Spec: krknv1alpha1.KrknOperatorTargetProviderSpec{OperatorName: "krkn-operator-acm", Active: true},
	}
	managedCluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata":   map[string]interface{}{"name": clusterName},
		"spec": map[string]interface{}{
			"managedClusterClientConfigs": []interface{}{
				map[string]interface{}{"url": "https://external.example.com:6443"},
			},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": managedClusterConditionAvailable, "status": "True"},
			},
		},
	}}
	managedCluster.SetGroupVersionKind(managedClusterGVK)
	manifestWork := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "work.open-cluster-management.io/v1",
		"kind":       "ManifestWork",
		"metadata":   map[string]interface{}{"name": ManifestWorkName, "namespace": clusterName},
	}}
	manifestWork.SetGroupVersionKind(manifestWorkGVK)
	proxyConfig := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "proxy.open-cluster-management.io/v1alpha1",
		"kind":       "ManagedProxyConfiguration",
		"metadata":   map[string]interface{}{"name": "cluster-proxy"},
		"spec": map[string]interface{}{
			"proxyServer": map[string]interface{}{"namespace": "multicluster-engine"},
		},
	}}
	proxyConfig.SetGroupVersionKind(proxyConfigGVK)
	proxyService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-proxy-addon-user", Namespace: "multicluster-engine",
			Labels: map[string]string{ProxyServiceLabel: ProxyServiceLabelValue},
		},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 9092}}},
	}

	reconciler := &KrknTargetRequestReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(request).
			WithObjects(request, providerObject, managedCluster, manifestWork, proxyConfig, proxyService).
			Build(),
		Scheme:            scheme,
		OperatorName:      "krkn-operator-acm",
		OperatorNamespace: "operator-system",
	}

	if _, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: request.Name, Namespace: request.Namespace},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &krknv1alpha1.KrknTargetRequest{}
	if err := reconciler.Get(ctx, types.NamespacedName{Name: request.Name, Namespace: request.Namespace}, updated); err != nil {
		t.Fatalf("failed to get reconciled request: %v", err)
	}
	targets := updated.Status.TargetData["krkn-operator-acm"]
	if len(targets) != 1 {
		t.Fatalf("target count = %d, want 1", len(targets))
	}
	target := targets[0]
	if target.ClusterAPIURL != proxyURL {
		t.Errorf("ClusterAPIURL = %q, want internal proxy URL %q", target.ClusterAPIURL, proxyURL)
	}
	if target.ClusterStatus != krknv1alpha1.ClusterStatusUnhealthy {
		t.Errorf("ClusterStatus = %q, want %q", target.ClusterStatus, krknv1alpha1.ClusterStatusUnhealthy)
	}
	if target.Online != nil {
		t.Errorf("Online = %v, want nil because liveness was not run", *target.Online)
	}
	if target.CheckedAt != nil {
		t.Errorf("CheckedAt = %v, want nil because liveness was not run", target.CheckedAt)
	}
}

func testLivenessKubeconfig(t *testing.T, serverURL string) string {
	t.Helper()
	config := clientcmdapi.NewConfig()
	config.Clusters["test"] = &clientcmdapi.Cluster{Server: serverURL}
	config.AuthInfos["test-user"] = &clientcmdapi.AuthInfo{}
	config.Contexts["test-context"] = &clientcmdapi.Context{Cluster: "test", AuthInfo: "test-user"}
	config.CurrentContext = "test-context"

	data, err := clientcmd.Write(*config)
	if err != nil {
		t.Fatalf("failed to write test kubeconfig: %v", err)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func TestGetConfiguredSecretName(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	_ = krknv1alpha1.AddToScheme(scheme)

	reconciler := &KrknTargetRequestReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	store := kvstore.Get()

	// Clean up before tests
	store.Delete("ACM_SECRET_LOCAL_CLUSTER")
	store.Delete("ACM_SECRET_MANAGED_CLUSTER_KRKN")
	store.Delete("ACM_SECRET_TEST_CLUSTER")

	tests := []struct {
		name               string
		clusterName        string
		configuredSecret   string
		setInConfigstore   bool
		expectedSecretName string
	}{
		{
			name:               "ConfigStore has custom secret",
			clusterName:        "local-cluster",
			configuredSecret:   "custom-secret",
			setInConfigstore:   true,
			expectedSecretName: "custom-secret",
		},
		{
			name:               "ConfigStore not set - use default",
			clusterName:        "managed-cluster-krkn",
			setInConfigstore:   false,
			expectedSecretName: ACMDefaultSecret,
		},
		{
			name:               "ConfigStore has empty value - use default",
			clusterName:        "test-cluster",
			configuredSecret:   "",
			setInConfigstore:   true,
			expectedSecretName: ACMDefaultSecret,
		},
		{
			name:               "ConfigStore has application-manager explicitly",
			clusterName:        "local-cluster",
			configuredSecret:   "application-manager",
			setInConfigstore:   true,
			expectedSecretName: "application-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup configstore
			varName := formatNamespaceToVarName(tt.clusterName)
			store.Delete(varName)

			if tt.setInConfigstore {
				store.SetValue(varName, tt.configuredSecret)
			}

			// Test
			result := reconciler.getConfiguredSecretName(ctx, tt.clusterName)

			// Verify
			if result != tt.expectedSecretName {
				t.Errorf("getConfiguredSecretName(%q) = %q, want %q",
					tt.clusterName, result, tt.expectedSecretName)
			}

			// Cleanup
			store.Delete(varName)
		})
	}
}

func TestGetClusterSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = krknv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		clusterName    string
		secretName     string
		createSecret   bool
		expectError    bool
		expectNotFound bool
	}{
		{
			name:           "Secret exists",
			clusterName:    "test-cluster",
			secretName:     "test-secret",
			createSecret:   true,
			expectError:    false,
			expectNotFound: false,
		},
		{
			name:           "Secret not found",
			clusterName:    "test-cluster",
			secretName:     "missing-secret",
			createSecret:   false,
			expectError:    true,
			expectNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake client
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

			if tt.createSecret {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.secretName,
						Namespace: tt.clusterName,
					},
					Data: map[string][]byte{
						"token": []byte("test-token"),
					},
				}
				clientBuilder = clientBuilder.WithObjects(secret)
			}

			reconciler := &KrknTargetRequestReconciler{
				Client: clientBuilder.Build(),
				Scheme: scheme,
			}

			// Test
			secret, err := reconciler.getClusterSecret(ctx, tt.clusterName, tt.secretName)

			// Verify
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.expectNotFound {
					if !errors.IsNotFound(err) {
						t.Errorf("expected NotFound error, got: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if secret == nil {
					t.Errorf("expected secret, got nil")
				}
			}
		})
	}
}
