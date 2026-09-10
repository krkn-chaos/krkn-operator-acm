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
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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

			target := buildClusterTargetWithLiveness(
				context.Background(),
				"managed-cluster",
				server.URL,
				testLivenessKubeconfig(t, server.URL),
			)

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
	if target.Online == nil || *target.Online {
		t.Fatal("Online should be false for an offline cluster")
	}
	if target.CheckedAt == nil || target.CheckedAt.IsZero() {
		t.Fatal("CheckedAt should be populated for an offline cluster")
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
